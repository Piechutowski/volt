package parser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func fileShape(f *ast.File, diags []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range f.Decls {
		fmt.Fprintf(&b, "%T %s+%d-%s+%d\n", d, d.Pos(), d.Pos().Offset(), d.End(), d.End().Offset())
		ast.Inspect(d, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				fmt.Fprintf(&b, "  %s@%s+%d\n", id.Name(), id.Pos(), id.Pos().Offset())
			}
			return true
		})
	}
	fmt.Fprintf(&b, "EOF %s+%d\n", f.EOF, f.EOF.Offset())
	for _, d := range diags {
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func snippetPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, dir := range []string{"valid", "invalid"} {
		filepath.WalkDir(filepath.Join("..", "conformance", "snippets", dir), func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".volt") {
				paths = append(paths, path)
			}
			return nil
		})
	}
	if len(paths) < 100 {
		t.Fatalf("only %d snippets found", len(paths))
	}
	return paths
}

func TestParseFileReuseIsTheParse(t *testing.T) {
	inserts := []string{
		"Table zz {\n  id int [pk]\n}\n",
		"  indented\n",
		"{\n",
		"}\n",
		"bare\n",
		"Ref: a.b >\n",
		"/* open\n",
		"'''\n",
		"// note\n",
	}
	check := func(name, path, src string, prev *parser.Reuse) *parser.Reuse {
		t.Helper()
		got, gotDiags, next, _ := parser.ParseFileReuse(path, src, prev)
		want, wantDiags, _, _ := parser.ParseFileReuse(path, src, nil)
		if a, b := fileShape(want, wantDiags), fileShape(got, gotDiags); a != b {
			t.Fatalf("%s: %s: the parse with reuse differs from the parse from nothing\n--- from nothing\n%s--- with reuse\n%s\n--- source\n%s", path, name, a, b, src)
		}
		return next
	}
	for _, path := range snippetPaths(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		_, _, base, _ := parser.ParseFileReuse(path, src, nil)
		texts := parser.ChunkTexts(base)
		offsets := []int{0}
		for _, text := range texts {
			offsets = append(offsets, offsets[len(offsets)-1]+len(text))
		}
		for i, off := range offsets {
			for _, ins := range inserts {
				edited := src[:off] + ins + src[off:]
				next := check("insert "+strconv.Quote(ins)+" at "+strconv.Itoa(off), path, edited, base)
				check("undo insert "+strconv.Quote(ins)+" at "+strconv.Itoa(off), path, src, next)
			}
			if i < len(texts) {
				edited := src[:off] + src[off+len(texts[i]):]
				next := check("delete element at "+strconv.Itoa(off), path, edited, base)
				check("undo delete element at "+strconv.Itoa(off), path, src, next)
			}
		}
	}
}

func TestParseFileReuseEdits(t *testing.T) {
	base := `package db

Type PostState string

Table posts {
  id     int [pk, increment]
  title  string
  status PostState  [default: 'active']
}

Table tags {
  id   int [pk, increment]
  name string    [unique]
}

Ref: tags.id > posts.id
`
	var prev *parser.Reuse
	step := func(name, src string, wantParsed, wantReused int) {
		t.Helper()
		f, diags, next, st := parser.ParseFileReuse("db/schema.volt", src, prev)
		prev = next
		fw, dw := parser.ParseFile("db/schema.volt", src)
		if a, b := fileShape(fw, dw), fileShape(f, diags); a != b {
			t.Fatalf("%s: differs from the whole-file parse\n--- whole\n%s--- reuse\n%s", name, a, b)
		}
		if st.Parsed != wantParsed || st.Reused != wantReused {
			t.Errorf("%s: parsed %d reused %d, want %d and %d", name, st.Parsed, st.Reused, wantParsed, wantReused)
		}
	}
	step("first", base, 5, 0)
	step("same", base, 0, 5)
	step("edit tags", strings.Replace(base, "name string", "name string    [note: 'edited']", 1), 1, 4)
	step("insert above", strings.Replace(base, "Table posts", "// a comment\n\nTable posts", 1), 2, 3)
	step("broken posts", strings.Replace(base, "title  string", "title  string [", 1), 2, 3)
	step("fixed", base, 1, 4)
	step("append", base+"\nTable extra {\n  id int [pk]\n}\n", 2, 4)
	step("whole rewrite", strings.ReplaceAll(base, "  ", "\t"), 3, 2)
}

func TestParseFileCRLFPositions(t *testing.T) {
	type at struct{ line, col, off int }
	positions := func(f *ast.File) []at {
		var out []at
		for _, d := range f.Decls {
			ast.Inspect(d, func(n ast.Node) bool {
				out = append(out, at{n.Pos().Line(), n.Pos().Column(), n.Pos().Offset()}, at{n.End().Line(), n.End().Column(), n.End().Offset()})
				return true
			})
		}
		return append(out, at{f.EOF.Line(), f.EOF.Column(), f.EOF.Offset()})
	}
	for _, path := range snippetPaths(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lf := strings.ReplaceAll(string(raw), "\r", "")
		crlf := strings.ReplaceAll(lf, "\n", "\r\n")
		f1, d1 := parser.ParseFile(path, lf)
		f2, d2 := parser.ParseFile(path, crlf)
		if a, b := diagsText(d1), diagsText(d2); a != b {
			t.Errorf("%s: diagnostics differ under CRLF\n--- LF\n%s--- CRLF\n%s", path, a, b)
			continue
		}
		p1, p2 := positions(f1), positions(f2)
		if len(p1) != len(p2) {
			t.Errorf("%s: %d positions under LF, %d under CRLF", path, len(p1), len(p2))
			continue
		}
		for i := range p1 {
			want := at{p1[i].line, p1[i].col, p1[i].off + p1[i].line - 1}
			if p2[i] != want {
				t.Errorf("%s: position %d is %+v under CRLF, want %+v", path, i, p2[i], want)
				break
			}
		}
	}
}

func diagsText(ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}
