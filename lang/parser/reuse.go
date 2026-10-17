package parser

import (
	"slices"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/scanner"
	"github.com/Piechutowski/volt/lang/token"
)

// Reuse is what a parse keeps for the next parse of the same file.
type Reuse struct {
	name   string
	chunks []*chunk
}

type chunk struct {
	text   string
	file   *token.File
	decls  []ast.Decl
	diags  []diag.Diagnostic
	eof    token.Position
	starts bool
	bound  bool
	lines  int
}

// ReuseStats counts one parse's work in declarations.
type ReuseStats struct {
	Reused, Parsed int
}

// ParseFileReuse parses src, reusing every chunk of prev (a previous
// parse of the same file, or nil) whose text stands unchanged at a
// place that is still a chunk's. It returns the file, its diagnostics,
// what to hand to the next parse, and the work done.
func ParseFileReuse(filename, src string, prev *Reuse) (*ast.File, []diag.Diagnostic, *Reuse, ReuseStats) {
	old := map[string][]*chunk{}
	if prev != nil && prev.name == filename {
		for _, c := range prev.chunks {
			k := lineFirst(c.text)
			old[k] = append(old[k], c)
		}
	}
	next := &Reuse{name: filename}
	f := &ast.File{Name: filename}
	var diags []diag.Diagnostic
	var st ReuseStats
	p := &parser{}
	pos, line := 0, 1
	for pos < len(src) {
		c := chunkReuse(old, src, pos)
		if c != nil {
			c.file.Relocate(pos, line)
			st.Reused++
		} else {
			c = p.chunkParse(token.NewChunk(filename, src[pos:], pos, line))
			st.Parsed++
		}
		next.chunks = append(next.chunks, c)
		f.Decls = append(f.Decls, c.decls...)
		diags = append(diags, c.diags...)
		f.EOF = c.eof
		pos += len(c.text)
		line += c.lines
	}
	if len(next.chunks) == 0 {
		c := p.chunkParse(token.NewChunk(filename, "", 0, 1))
		next.chunks = append(next.chunks, c)
		f.EOF = c.eof
		st.Parsed++
	}
	diag.Sort(diags)
	return f, diags, next, st
}

func chunkReuse(old map[string][]*chunk, src string, pos int) *chunk {
	rest := src[pos:]
	key := lineFirst(rest)
	for i, c := range old[key] {
		if !strings.HasPrefix(rest, c.text) || (pos > 0 && !c.starts) {
			continue
		}
		after := rest[len(c.text):]
		if c.bound {
			if after == "" || !scanner.StartsDecl(after) {
				continue
			}
		} else if after != "" {
			continue
		}
		old[key] = slices.Delete(old[key], i, i+1)
		return c
	}
	return nil
}

func (p *parser) chunkParse(f *token.File) *chunk {
	toks, errs, end, bound := scanner.ScanChunk(f, p.buf)
	f.Src = f.Src[:end]
	p.toks, p.pos, p.diags = toks, 0, errs
	parsed := p.file(f.Name)
	p.buf = toks[:0]
	return &chunk{
		text: f.Src, file: f, decls: parsed.Decls, diags: p.diags, eof: parsed.EOF,
		starts: toks[0].DeclStart && toks[0].Pos == f.At(0, 1, 1),
		bound:  bound,
		lines:  strings.Count(f.Src, "\n"),
	}
}

func lineFirst(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i+1]
	}
	return text
}
