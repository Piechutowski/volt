package parser_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// fileShape renders what a parse produced, positions included, so two
// parses can be compared: declaration kinds and spans, the EOF, and
// every diagnostic.
func fileShape(f *ast.File, diags []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range f.Decls {
		fmt.Fprintf(&b, "%T %s-%s\n", d, d.Pos(), d.End())
		ast.Inspect(d, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				fmt.Fprintf(&b, "  %s@%s\n", id.Name(), id.Pos())
			}
			return true
		})
	}
	fmt.Fprintf(&b, "EOF %s\n", f.EOF)
	for _, d := range diags {
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// TestParseFileReuseMatchesParseFile proves the chunked parse of every
// conformance snippet, valid and invalid, is the whole-file parse:
// same declarations at the same positions, same diagnostics (D83).
func TestParseFileReuseMatchesParseFile(t *testing.T) {
	var paths []string
	for _, dir := range []string{"valid", "invalid"} {
		filepath.WalkDir(filepath.Join("..", "conformance", "snippets", dir), func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(path, ".volt") || strings.HasSuffix(path, ".dbml")) {
				paths = append(paths, path)
			}
			return nil
		})
	}
	if len(paths) < 100 {
		t.Fatalf("only %d snippets found", len(paths))
	}
	differ := 0
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f1, d1 := parser.ParseFile(path, string(src))
		f2, d2, _, _ := parser.ParseFileReuse(path, string(src), nil)
		if a, b := fileShape(f1, d1), fileShape(f2, d2); a != b {
			differ++
			if differ <= 3 {
				t.Errorf("%s: chunked parse differs\n--- whole\n%s--- chunked\n%s", path, a, b)
			}
		}
	}
	if differ > 3 {
		t.Errorf("%d snippets differ in all", differ)
	}
}

// TestParseFileReuseEdits proves that an edit re-parses the changed
// declaration only and the result is the whole-file parse of the new
// text, edit by edit, through valid and broken states.
func TestParseFileReuseEdits(t *testing.T) {
	base := `package db

Enum status {
  active
  retired
}

Table posts {
  id     integer [pk, increment]
  title  text    [not null]
  status status  [not null, default: status.active]
}

Table tags {
  id   integer [pk, increment]
  name text    [not null, unique]
}

Ref: posts.id < tags.id
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
	step("edit tags", strings.Replace(base, "name text", "name text    [note: 'edited']", 1), 1, 4)
	// The comment joins the enum's chunk, and the tags chunk reverts.
	step("insert above", strings.Replace(base, "Table posts", "// a comment\n\nTable posts", 1), 2, 3)
	step("broken posts", strings.Replace(base, "title  text", "title  text [", 1), 2, 3)
	step("fixed", base, 1, 4)
	step("append", base+"\nTable extra {\n  id integer [pk]\n}\n", 2, 4)
	// The package clause has no double space: its chunk survives.
	step("whole rewrite", strings.ReplaceAll(base, "  ", "\t"), 4, 1)
}
