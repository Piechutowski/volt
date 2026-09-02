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

	File  *ast.File
	Info  *check.Info
	Diags []diag.Diagnostic
	Index *Index

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

// Update replaces the document text and re-runs the whole front end.
func (d *Document) Update(text string) {
	d.Text = text
	d.lineOffsets = d.lineOffsets[:0]
	d.lineOffsets = append(d.lineOffsets, 0)
	for i, b := range []byte(text) {
		if b == '\n' {
			d.lineOffsets = append(d.lineOffsets, i+1)
		}
	}

	file, diags := parser.ParseFile(pathFromURI(d.URI), text)
	info, semDiags := check.File(file)
	// Info always comes from the single-file pass — hover, definition
	// and completion read it. Diagnostics depend on where the file
	// lives: inside a Volt project the whole-project check is the truth
	// (this file's tables may be half of a package, §V1.5) and the
	// single-file verdicts would be wrong; alone, the DBML pass stands.
	if projDiags, ok := d.voltProjectDiags(); ok {
		diags = projDiags
	} else {
		diags = append(diags, semDiags...)
		// vet warnings only make sense on files that already check clean;
		// stacking style advice on top of hard errors is noise while typing.
		if !diag.HasErrors(diags) {
			diags = append(diags, vet.Run(file, info, analyzersActive()...)...)
		}
	}
	diag.Sort(diags)

	d.File = file
	d.Info = info
	d.Diags = diags
	d.Index = NewIndex(file, info)
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
func (d *Document) lineText(line int) string {
	if line < 0 || line >= len(d.lineOffsets) {
		return ""
	}
	start := d.lineOffsets[line]
	end := len(d.Text)
	if line+1 < len(d.lineOffsets) {
		end = d.lineOffsets[line+1] - 1
	}
	if start > end {
		return ""
	}
	return d.Text[start:end]
}

// ToLSP converts a front-end position to an LSP position.
func (d *Document) ToLSP(p token.Position) protocol.Position {
	line := p.Line - 1
	if line < 0 {
		return protocol.Position{}
	}
	text := d.lineText(line)
	col := 0
	need := p.Column - 1
	for _, r := range text {
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
	start := d.ToLSP(p)
	off := p.Offset
	end := off
	for end < len(d.Text) && isIdentByte(d.Text[end]) {
		end++
	}
	if end == off && off < len(d.Text) && d.Text[off] != '\n' {
		end = off + 1
	}
	width := end - off
	return protocol.Range{
		Start: start,
		End:   protocol.Position{Line: start.Line, Character: start.Character + protocol.UInteger(width)},
	}
}
