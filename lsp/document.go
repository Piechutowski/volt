// Package lsp implements the Volt language server — one server for the
// whole language, of which the DBML schema core is the inner layer (docs/spec.md
// §V0).
package lsp

import (
	"net/url"
	"strings"
	"unicode/utf16"

	"github.com/Piechutowski/volt/lang"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
	"github.com/Piechutowski/volt/lang/vet"
)

// Document is one open text document plus everything derived from it.
type Document struct {
	URI  string
	Text string

	// Siblings, when set, snapshots every open buffer keyed by on-disk
	// path. Nil means only this document overlays its saved file.
	Siblings func() map[string]string
	// Session, when set, returns the caches of a project root (D79).
	// Nil means a fresh analysis every time.
	Session func(root string) *lang.Session

	File  *ast.File
	Info  *check.Info
	Diags []diag.Diagnostic
	Index *Index

	// local is the file's own verdict — parse, single-file check, and
	// vet on a clean check once vetLocal has run — which stands when
	// the file belongs to no project package; inside one, the project
	// analysis is the truth.
	local []diag.Diagnostic
	// clean says the file's own parse and check found no error; vetted
	// says vetLocal has added the vet advice to local for this text.
	clean, vetted bool
	// resGen is the analysis generation the project fields came from,
	// for a server-driven document (adopt).
	resGen int

	// The document's own front end is memoized per document (D103):
	// the parse (D83), the check (D84) and the index (D85). The three
	// belong to the goroutine that updates the document, the handler's
	// for a server document (D79, D96); the session's own handles are
	// the session's.
	reuse      *parser.Reuse
	schema     check.Memo
	index      IndexMemo
	parseStats parser.ReuseStats

	// vindex is the project-wide Volt symbol graph, rebuilt with the
	// diagnostics whenever this file belongs to a Volt project. Nil for
	// a file outside any project.
	vindex *voltIndex
	// vpkg is the project package containing this file (nil outside a
	// project).
	vpkg *lang.Package

	lineOffsets []int // byte offset of the start of each line
}

// NewDocument analyzes text and returns a ready document.
func NewDocument(uri, text string) *Document {
	d := &Document{URI: uri}
	d.Update(text)
	return d
}

// Update replaces the document text and re-runs the whole front end:
// the file's own pass, then the project pass. Inside a Volt project
// the whole-project check is the truth (§V1.5); alone, the DBML pass
// stands.
func (d *Document) Update(text string) {
	d.UpdateLocal(text)
	if projDiags, ok := d.voltProjectDiags(); ok {
		d.Diags = projDiags
	} else {
		d.vetLocal()
	}
	d.resGen = 0 // whatever the server has, this is at least as fresh
}

// UpdateLocal replaces the text and re-runs the file's own front end —
// parse, single-file check, index — through the document's own memos
// (D103). The project pass is Update's, or the server's background
// analysis (D79); until either runs, the project fields keep what
// they had. The vet advice is not part of this: it is added by
// vetLocal.
func (d *Document) UpdateLocal(text string) {
	d.textSet(text)
	var diags []diag.Diagnostic
	d.File, diags, d.reuse, d.parseStats = parser.ParseFileReuse(pathFromURI(d.URI), text, d.reuse)
	info, semDiags := check.FileMemo(d.File, &d.schema)
	d.Info = info
	diags = append(diags, semDiags...)
	diag.Sort(diags)
	d.local = diags
	d.clean = !diag.HasErrors(diags)
	d.vetted = false
	d.Index = NewIndexMemo(d.File, d.Info, &d.index)
	if d.vindex == nil {
		d.Diags = d.local
	}
}

// vetLocal adds the single-file vet advice to the document's own
// verdict, once per text, and makes that verdict the document's when
// no project analysis stands. It runs only where the file's own
// verdict is the truth: a file outside any project, or one under a
// project the loader never read (D103). Vet advice comes only on a
// clean check.
func (d *Document) vetLocal() {
	if !d.vetted {
		d.vetted = true
		if d.clean {
			d.local = append(d.local, vet.Run(d.File, d.Info, analyzersActive()...)...)
			diag.Sort(d.local)
		}
	}
	if d.vindex == nil {
		d.Diags = d.local
	}
}

// textSet replaces the text and its line table.
func (d *Document) textSet(text string) {
	d.Text = text
	d.lineOffsets = lineStarts(text)
}

// localVerdict is the single-file verdict of a text no document
// memoizes: parse, check, and vet advice when the file checks clean.
// The background analysis computes it off the handler goroutine and
// so off the document (D79). The diagnostics come sorted.
func localVerdict(path, text string) []diag.Diagnostic {
	file, diags := parser.ParseFile(path, text)
	info, semDiags := check.File(file)
	diags = append(diags, semDiags...)
	if !diag.HasErrors(diags) {
		diags = append(diags, vet.Run(file, info, analyzersActive()...)...)
	}
	diag.Sort(diags)
	return diags
}

// lineStarts returns the byte offset at which each line of text begins.
func lineStarts(text string) []int {
	starts := make([]int, 1, strings.Count(text, "\n")+1)
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// GoFilesChanged reports whether the Go files this document's project
// analysis read have changed on disk since; the checker's D63 facts
// are then stale. Only Go files that were actually scanned count.
func (d *Document) GoFilesChanged() bool {
	if d.vindex == nil {
		return false
	}
	return d.vindex.goStale()
}

// analyzersActive is every registered vet analyzer except modelname: the
// single-file pass this feeds does not resolve the [model:] setting it
// wants (see docs/editor.md).
func analyzersActive() []vet.Analyzer {
	var out []vet.Analyzer
	for _, a := range vet.All() {
		if a.Name() != "modelname" {
			out = append(out, a)
		}
	}
	return out
}

// pathFromURI converts a file URI to the on-disk path; the result is a
// real lookup key, so it must be the decoded path.
func pathFromURI(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		return u.Path
	}
	return strings.TrimPrefix(uri, "file://")
}

// ---------------------------------------------------------------------------
// Position conversion. token.Position counts 1-based lines and 1-based rune
// columns; LSP counts 0-based lines and 0-based UTF-16 code units.

// lineText returns the 0-based line without its trailing newline.
func (d *Document) lineText(line int) string { return lineTextIn(d.Text, d.lineOffsets, line) }

// lineTextIn is lineText over any text and its line table.
func lineTextIn(text string, starts []int, line int) string {
	if line < 0 || line >= len(starts) {
		return ""
	}
	start := starts[line]
	end := len(text)
	if line+1 < len(starts) {
		end = starts[line+1] - 1
	}
	if start > end {
		return ""
	}
	return text[start:end]
}

// ToLSP converts a front-end position to an LSP position.
func (d *Document) ToLSP(p token.Position) protocol.Position {
	return lspPosition(d.Text, d.lineOffsets, p)
}

// lspPosition is ToLSP over any text and its line table.
func lspPosition(text string, starts []int, p token.Position) protocol.Position {
	line := int(p.Line()) - 1
	if line < 0 {
		return protocol.Position{}
	}
	col := 0
	need := int(p.Column()) - 1
	for _, r := range lineTextIn(text, starts, line) {
		if need <= 0 {
			break
		}
		col += len(utf16.Encode([]rune{r}))
		need--
	}
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

// FromLSP converts an LSP position to a byte offset into d.Text. A
// line past the end of the text is its end; a character past the end
// of a line is the end of that line's text, before its line break,
// a carriage return included.
func (d *Document) FromLSP(pos protocol.Position) int {
	line := int(pos.Line)
	if line >= len(d.lineOffsets) {
		return len(d.Text)
	}
	text := strings.TrimSuffix(d.lineText(line), "\r")
	need := int(pos.Character)
	byteCol := 0
	for _, r := range text {
		if need <= 0 {
			break
		}
		u := len(utf16.Encode([]rune{r}))
		if u > need {
			break
		}
		need -= u
		byteCol += len(string(r))
	}
	return d.lineOffsets[line] + byteCol
}

// RangeOf converts a node's extent to an LSP range.
func (d *Document) RangeOf(n ast.Node) protocol.Range {
	return protocol.Range{Start: d.ToLSP(n.Pos()), End: d.ToLSP(n.End())}
}

// tokenRange converts a token's extent to an LSP range.
func (d *Document) tokenRange(t token.Token) protocol.Range {
	return protocol.Range{Start: d.ToLSP(t.Pos), End: d.ToLSP(t.End())}
}

// isIdentByte loosely matches identifier bytes for diagnostic ranges.
func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// diagnosticRange widens a diagnostic's point position to the token-ish
// span starting there.
func (d *Document) diagnosticRange(p token.Position) protocol.Range {
	return diagnosticRangeIn(d.Text, d.lineOffsets, p)
}

// diagnosticRangeIn is diagnosticRange over any text and its line table.
func diagnosticRangeIn(text string, starts []int, p token.Position) protocol.Range {
	start := lspPosition(text, starts, p)
	off := int(p.Offset())
	end := off
	for end < len(text) && isIdentByte(text[end]) {
		end++
	}
	if end == off && off < len(text) && text[off] != '\n' {
		end = off + 1
	}
	width := end - off
	return protocol.Range{
		Start: start,
		End:   protocol.Position{Line: start.Line, Character: start.Character + protocol.UInteger(width)},
	}
}
