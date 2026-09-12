// Package align lays out the column-aligned blocks gofmt produces —
// struct fields, const and var specs — the way go/printer's tabwriter
// does, so a generator writes gofmt-canonical text without running
// gofmt (D75).
//
// The rules are text/tabwriter's as go/printer configures it (padding
// one, spaces): a line is a sequence of cells, every cell but the last
// tab-terminated; a column's block is a maximal run of consecutive
// lines that carry a terminated cell in that column; each terminated
// cell is padded to the widest cell of its block plus one space; a line
// without cells — blank, or a comment — ends every block. Widths count
// runes, as tabwriter does.
package align

import (
	"strings"
	"unicode/utf8"
)

// Block is one aligned block: rows of cells and verbatim lines, in
// order.
type Block struct {
	lines []line
}

type line struct {
	cells []string // nil for a verbatim line
	text  string
}

// Row adds a line of cells: for a struct field name, type and tag; for
// a spec the name, an optional type, and "= value".
func (b *Block) Row(cells ...string) {
	b.lines = append(b.lines, line{cells: cells})
}

// Line adds a verbatim line — a comment, or "" for a blank line — which
// ends every column block.
func (b *Block) Line(text string) {
	b.lines = append(b.lines, line{text: text})
}

// Len reports how many lines the block holds.
func (b *Block) Len() int { return len(b.lines) }

// WriteTo writes the block, each line prefixed by indent and terminated
// by a newline.
func (b *Block) WriteTo(w *strings.Builder, indent string) {
	n := len(b.lines)
	pad := make([][]int, n)
	maxCells := 0
	for i, l := range b.lines {
		pad[i] = make([]int, len(l.cells))
		if len(l.cells) > maxCells {
			maxCells = len(l.cells)
		}
	}
	for c := 0; c < maxCells-1; c++ {
		for i := 0; i < n; {
			if len(b.lines[i].cells) <= c+1 {
				i++
				continue
			}
			j, width := i, 0
			for j < n && len(b.lines[j].cells) > c+1 {
				if w := utf8.RuneCountInString(b.lines[j].cells[c]); w > width {
					width = w
				}
				j++
			}
			for k := i; k < j; k++ {
				pad[k][c] = width + 1
			}
			i = j
		}
	}
	for i, l := range b.lines {
		if l.cells == nil {
			if l.text != "" {
				w.WriteString(indent)
				w.WriteString(l.text)
			}
			w.WriteByte('\n')
			continue
		}
		w.WriteString(indent)
		for c, cell := range l.cells {
			w.WriteString(cell)
			if c < len(l.cells)-1 {
				for k := utf8.RuneCountInString(cell); k < pad[i][c]; k++ {
					w.WriteByte(' ')
				}
			}
		}
		w.WriteByte('\n')
	}
}

// Finish returns a generated file's text with every run of blank lines
// collapsed to one and exactly one trailing newline, as gofmt leaves
// it. Everything else about the layout is the emitter's job.
func Finish(text string) []byte {
	text = strings.TrimRight(text, "\n") + "\n"
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return []byte(text)
}
