package ast_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestChildrenAgreesWithInspect(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../conformance/snippets/*/*.dbml", "../conformance/snippets/*/*.volt", "../conformance/snippets/*/*/*.volt", "../../grammar/examples/*"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 100 {
		t.Fatalf("only %d corpus files", len(files))
	}
	texts := map[string]string{}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		texts[path] = string(src)
	}
	for name, text := range corpus.Files(corpus.Spec{Packages: 2, Tables: 30, Columns: 8}) {
		texts["corpus/"+name] = text
	}
	nodes := 0
	for path, text := range texts {
		f, _ := parser.ParseFile(path, text)
		var byInspect, byChildren []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			byInspect = append(byInspect, n)
			return true
		})
		var walk func(n ast.Node)
		walk = func(n ast.Node) {
			byChildren = append(byChildren, n)
			for _, c := range ast.Children(n) {
				walk(c)
			}
		}
		walk(f)
		if len(byInspect) != len(byChildren) {
			t.Fatalf("%s: Inspect visits %d nodes, Children %d", path, len(byInspect), len(byChildren))
		}
		for i := range byInspect {
			if byInspect[i] != byChildren[i] {
				t.Fatalf("%s: node %d differs: Inspect %T, Children %T", path, i, byInspect[i], byChildren[i])
			}
		}
		nodes += len(byInspect)
	}
	if nodes < 5000 {
		t.Fatalf("only %d nodes walked", nodes)
	}
}
