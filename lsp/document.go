// Package lsp implements the Volt language server — one server for the
// whole language, of which the schema core is the inner layer (docs/spec.md
// §layers-files-and-the-superset-rule).
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
	// Session, when set, returns the caches of a project root.
	// Nil means a fresh analysis every time.
	Session func(root string) *lang.Session

	File  *ast.File
	Info  *check.Info
	Diags []diag.Diagnostic
	Index *Index

	local         []diag.Diagnostic
	clean, vetted bool
	resGen        int

	reuse      *parser.Reuse
	schema     check.Memo
	index      IndexMemo
	parseStats parser.ReuseStats

	vindex *voltIndex
	vpkg   *lang.Package

	lineOffsets []int
}

// NewDocument analyzes text and returns a ready document.
func NewDocument(uri, text string) *Document {
	d := &Document{URI: uri}
	d.Update(text)
	return d
}

// Update replaces the document text and re-runs the whole front end:
// the file's own pass, then the project pass. Inside a Volt project
// the whole-project check is the truth (§projects-and-packages); alone, the schema pass
// stands.
func (d *Document) Update(text string) {
	d.UpdateLocal(text)
	if projDiags, ok := d.voltProjectDiags(); ok {
		d.Diags = projDiags
	} else {
		d.vetLocal()
	}
	d.resGen = 0
}

// UpdateLocal replaces the text and re-runs the file's own front end —
// parse, single-file check, index — through the document's own memos
// . The project pass is Update's, or the server's background
// analysis; until either runs, the project fields keep what
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

func (d *Document) textSet(text string) {
	d.Text = text
	d.lineOffsets = lineStarts(text)
}

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
// analysis read have changed on disk since; the checker's Go-reference facts
// are then stale. Only Go files that were actually scanned count.
func (d *Document) GoFilesChanged() bool {
	if d.vindex == nil {
		return false
	}
	return d.vindex.goStale()
}

func analyzersActive() []vet.Analyzer {
	var out []vet.Analyzer
	for _, a := range vet.All() {
		if a.Name() != "modelname" {
			out = append(out, a)
		}
	}
	return out
}

func pathFromURI(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		return u.Path
	}
	return strings.TrimPrefix(uri, "file://")
}

func (d *Document) lineText(line int) string { return lineTextIn(d.Text, d.lineOffsets, line) }

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

func (d *Document) tokenRange(t token.Token) protocol.Range {
	return protocol.Range{Start: d.ToLSP(t.Pos), End: d.ToLSP(t.End())}
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

func (d *Document) diagnosticRange(p token.Position) protocol.Range {
	return diagnosticRangeIn(d.Text, d.lineOffsets, p)
}

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
