package parser

// Parsing with reuse (D83, PERF-10): a file is split into chunks, one
// per top-level declaration, each scanned and parsed as a token.File
// of its own with a base offset and line in the whole. The next parse
// of the same file keeps every chunk whose text is unchanged — the
// declaration's nodes and diagnostics as they are, relocated by one
// store of the base — and parses only the chunks that changed. Top-
// level declarations are independent syntactically, so a chunk's parse
// is the same whatever surrounds it; the checker sees the merged
// declarations as always.

import (
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
	text  string
	file  *token.File
	decls []ast.Decl
	diags []diag.Diagnostic
	eof   token.Position
}

// ReuseStats counts one parse's work in declarations.
type ReuseStats struct {
	Reused, Parsed int
}

// ParseFileReuse parses src as ParseFile does, reusing every chunk of
// prev (a previous parse of the same file, or nil) whose text is
// unchanged. It returns the file, its diagnostics, what to hand to the
// next parse, and the work done.
func ParseFileReuse(filename, src string, prev *Reuse) (*ast.File, []diag.Diagnostic, *Reuse, ReuseStats) {
	if strings.IndexByte(src, '\r') >= 0 {
		src = strings.ReplaceAll(src, "\r", "")
	}
	spans := chunkSplit(src)
	old := map[string][]*chunk{}
	if prev != nil && prev.name == filename {
		for _, c := range prev.chunks {
			old[c.text] = append(old[c.text], c)
		}
	}
	next := &Reuse{name: filename, chunks: make([]*chunk, 0, len(spans))}
	f := &ast.File{Name: filename}
	var diags []diag.Diagnostic
	var st ReuseStats
	for _, sp := range spans {
		text := src[sp.start:sp.end]
		var c *chunk
		if cs := old[text]; len(cs) > 0 {
			c, old[text] = cs[0], cs[1:]
			c.file.Relocate(sp.start, sp.line)
			st.Reused++
		} else {
			c = chunkParse(token.NewChunk(filename, text, sp.start, sp.line))
			st.Parsed++
		}
		next.chunks = append(next.chunks, c)
		f.Decls = append(f.Decls, c.decls...)
		diags = append(diags, c.diags...)
		f.EOF = c.eof
	}
	if len(spans) == 0 {
		c := chunkParse(token.NewChunk(filename, "", 0, 1))
		next.chunks = append(next.chunks, c)
		f.EOF = c.eof
		st.Parsed++
	}
	diag.Sort(diags)
	return f, diags, next, st
}

// chunkParse scans and parses one chunk's declarations.
func chunkParse(file *token.File) *chunk {
	toks, errs := scanner.ScanFile(file)
	p := &parser{toks: toks, diags: errs}
	parsed := p.file(file.Name)
	return &chunk{text: file.Src, file: file, decls: parsed.Decls, diags: p.diags, eof: parsed.EOF}
}

// chunkSpan is one chunk of the source: byte range and 1-based line.
type chunkSpan struct {
	start, end, line int
}

// chunkSplit cuts src at every line that begins a top-level
// declaration: a letter in the first column, outside every brace,
// string and comment. Whatever precedes the first declaration (the
// package clause, comments) is the first chunk; blank and comment
// lines between declarations belong to the preceding one. The lexical
// state it tracks is the scanner's: braces, 'strings', ”'multi-line
// strings”', "quoted identifiers", `function expressions`, line and
// block comments. A split the parser then disagrees with only moves a
// syntax error to the chunk boundary, where the parser resynchronizes
// anyway.
func chunkSplit(src string) []chunkSpan {
	var spans []chunkSpan
	depth := 0
	var quote byte // ' " ` or 0
	multi := false // inside '''...'''
	block := false // inside /* */
	start := 0
	for i := 0; i < len(src); {
		// The line's start decides whether it opens a chunk.
		if i > start && depth == 0 && quote == 0 && !block && isLetterByte(src[i]) {
			spans = append(spans, chunkSpan{start: start, end: i, line: lineOf(src, start, spans)})
			start = i
		}
		// Consume the line, tracking state.
		for i < len(src) {
			c := src[i]
			if c == '\n' {
				i++
				break
			}
			switch {
			case block:
				if c == '*' && i+1 < len(src) && src[i+1] == '/' {
					block = false
					i += 2
					continue
				}
			case multi:
				if strings.HasPrefix(src[i:], "'''") {
					multi = false
					i += 3
					continue
				}
			case quote != 0:
				if c == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				if c == quote {
					quote = 0
				}
			case c == '/' && i+1 < len(src) && src[i+1] == '/':
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			case c == '/' && i+1 < len(src) && src[i+1] == '*':
				block = true
				i += 2
				continue
			case c == '\'':
				if strings.HasPrefix(src[i:], "'''") {
					multi = true
					i += 3
					continue
				}
				quote = '\''
			case c == '"' || c == '`':
				quote = c
			case c == '{':
				depth++
			case c == '}':
				if depth > 0 {
					depth--
				}
			}
			i++
		}
	}
	if start < len(src) || len(spans) == 0 {
		spans = append(spans, chunkSpan{start: start, end: len(src), line: lineOf(src, start, spans)})
	}
	return spans
}

// lineOf is the 1-based line at byte offset start, counted from the
// previous span's start so the split stays linear.
func lineOf(src string, start int, spans []chunkSpan) int {
	if len(spans) == 0 {
		return 1 + strings.Count(src[:start], "\n")
	}
	prev := spans[len(spans)-1]
	return prev.line + strings.Count(src[prev.start:start], "\n")
}

func isLetterByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}
