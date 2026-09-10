// Package lsp implements the Volt language server — one server for the
// whole language, of which the DBML schema core is the inner layer (docs/spec.md
// §V0). It wraps the front end (scanner, parser, check, vet, and the
// lang project checker for .volt projects) behind the Language Server
// Protocol:
// diagnostics, completion, hover, definition, references, rename and
// document symbols. One Document per open file holds the parse result, the
// semantic model and the symbol occurrence index.
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
	// path. The Volt project pass overlays them all, so cross-file
	// analysis sees unsaved edits in other tabs. Nil (a lone document,
	// as in tests) means only this document overlays its saved file.
	Siblings func() map[string]string
	// Session, when set, returns the caches of a project root (D79):
	// the project pass loads and checks through them. Nil means a
	// fresh analysis every time (a lone document, as in tests).
	Session func(root string) *lang.Session

	File  *ast.File
	Info  *check.Info
	Diags []diag.Diagnostic
	Index *Index

	// local is the file's own verdict — parse, single-file check, vet
	// on a clean check — which stands when the file belongs to no
	// project; inside one, the project analysis is the truth.
	local []diag.Diagnostic
	// resGen is the analysis generation the project fields came from,
	// for a server-driven document (adopt).
	resGen int

	// vindex is the project-wide Volt symbol graph, rebuilt with the
	// diagnostics whenever this file belongs to a Volt project. Nil for
	// a file outside any project (navigation then stays single-file).
	vindex *voltIndex
	// vpkg is the project package containing this file (nil outside a
	// project): completion reads its imports and sibling declarations.
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
// the file's own pass, then the project pass. Diagnostics depend on
// where the file lives: inside a Volt project the whole-project check
// is the truth (this file's tables may be half of a package, §V1.5)
// and the single-file verdicts would be wrong; alone, the DBML pass
// stands.
func (d *Document) Update(text string) {
	d.UpdateLocal(text)
	if projDiags, ok := d.voltProjectDiags(); ok {
		d.Diags = projDiags
	}
	d.resGen = 0 // whatever the server has, this is at least as fresh
}

// UpdateLocal replaces the text and re-runs the file's own front end —
// parse, single-file check, index — which hover, definition and
// completion read. The project pass is Update's, or the server's
// background analysis (D79); until either runs, the project fields
// keep what they had.
func (d *Document) UpdateLocal(text string) {
	d.textSet(text)
	d.File, d.Info, d.local = localAnalyze(pathFromURI(d.URI), text)
	d.Index = NewIndex(d.File, d.Info)
	if d.vindex == nil {
		d.Diags = d.local
	}
}

// textSet replaces the text and its line table.
func (d *Document) textSet(text string) {
	d.Text = text
	d.lineOffsets = lineStarts(text)
}

// localAnalyze is the single-file front end: parse, check, and vet
// advice when the file checks clean — style notes stacked on hard
// errors are noise while typing. The diagnostics come sorted.
func localAnalyze(path, text string) (*ast.File, *check.Info, []diag.Diagnostic) {
	file, diags := parser.ParseFile(path, text)
	info, semDiags := check.File(file)
	diags = append(diags, semDiags...)
	if !diag.HasErrors(diags) {
		diags = append(diags, vet.Run(file, info, analyzersActive()...)...)
	}
	diag.Sort(diags)
	return file, info, diags
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
// analysis read have changed on disk since — the checker's D63 facts
// and the Go-reference navigation are then stale until Update runs.
// Only Go files that were actually scanned count: a project without
// Go references never re-runs for this reason.
func (d *Document) GoFilesChanged() bool {
	if d.vindex == nil {
		return false
	}
	return d.vindex.goStale()
}

// analyzersActive is every registered vet analyzer except modelname: the
// [model:] setting it wants is above the DBML layer, and the single-file
// pass this feeds does not resolve it (see docs/editor.md).
func analyzersActive() []*vet.Analyzer {
	var out []*vet.Analyzer
	for _, a := range vet.All() {
		if a.Name != "modelname" {
			out = append(out, a)
		}
	}
	return out
}

// pathFromURI converts a file URI to the on-disk path. Editors
// percent-encode URIs (spaces, non-ASCII), and the result is a real
// lookup key — the project overlay and the diagnostic filter match it
// against filepath.WalkDir paths — so it must be the decoded path, not
// a cosmetic trim.
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

// lspPosition is ToLSP over any text and its line table, so the
// background analysis can position diagnostics in the text it saw.
func lspPosition(text string, starts []int, p token.Position) protocol.Position {
	line := p.Line - 1
	if line < 0 {
		return protocol.Position{}
	}
	col := 0
	need := p.Column - 1
	for _, r := range lineTextIn(text, starts, line) {
		if need <= 0 {
			break
		}
		col += len(utf16.Encode([]rune{r}))
		need--
	}
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

// FromLSP converts an LSP position to a byte offset into d.Text.
func (d *Document) FromLSP(pos protocol.Position) int {
	line := int(pos.Line)
	if line >= len(d.lineOffsets) {
		return len(d.Text)
	}
	text := d.lineText(line)
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
// span starting there, so editors underline something visible.
func (d *Document) diagnosticRange(p token.Position) protocol.Range {
	return diagnosticRangeIn(d.Text, d.lineOffsets, p)
}

// diagnosticRangeIn is diagnosticRange over any text and its line table.
func diagnosticRangeIn(text string, starts []int, p token.Position) protocol.Range {
	start := lspPosition(text, starts, p)
	off := p.Offset
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
