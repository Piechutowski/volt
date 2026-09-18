package vet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// walkSources are the texts the walk and memo tests run over: every
// analyzer's testdata and a generated project of every construct.
func walkSources(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, pattern := range []string{"*.dbml", "*.volt"} {
		files, err := filepath.Glob(filepath.Join("testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			out[file] = string(src)
		}
	}
	for name, src := range corpus.Files(corpus.Spec{Packages: 2, Tables: 6, Columns: 5}) {
		out["corpus/"+name] = src
	}
	return out
}

// TestNodesOfMatchesInspect proves the pre-order list the rules walk
// is the sequence ast.Inspect visits, declaration by declaration.
func TestNodesOfMatchesInspect(t *testing.T) {
	for name, src := range walkSources(t) {
		f, _ := parser.ParseFile(name, src)
		for _, d := range f.Decls {
			if d == nil {
				continue
			}
			var want []ast.Node
			ast.Inspect(d, func(n ast.Node) bool {
				if n != nil {
					want = append(want, n)
				}
				return true
			})
			got := nodesOf(d)
			if len(got) != len(want) {
				t.Fatalf("%s: %T at %s: %d nodes, Inspect visits %d", name, d, d.Pos(), len(got), len(want))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("%s: %T at %s: node %d is %T at %s, Inspect visits %T at %s", name, d, d.Pos(), i, got[i], got[i].Pos(), want[i], want[i].Pos())
				}
			}
		}
	}
}

func diagsText(ds []diag.Diagnostic) string {
	var b strings.Builder
	for _, dg := range ds {
		b.WriteString(dg.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// TestMemoMatchesFreshRun proves a run through the memo, edit after
// edit, warns what a fresh run warns, and that an unchanged file
// answers every declaration from the memo while an edit inside one
// declaration judges that declaration and the ones whose facts name
// it.
func TestMemoMatchesFreshRun(t *testing.T) {
	text := corpus.Files(corpus.Spec{Tables: 6, Columns: 5, Single: true})["schema.volt"]
	var reuse *parser.Reuse
	var schema check.Memo
	var memo Memo
	round := func(name, text string, misses int) {
		t.Helper()
		f, diags, next, _ := parser.ParseFileReuse("schema.volt", text, reuse)
		reuse = next
		info, semDiags := check.FileMemo(f, &schema)
		if diag.HasErrors(append(diags, semDiags...)) {
			t.Fatalf("%s: %v %v", name, diags, semDiags)
		}
		got := diagsText(RunMemo(f, info, nil, &memo))
		if want := diagsText(Run(f, info)); got != want {
			t.Fatalf("%s: the memoized run differs from a fresh one\n--- memo\n%s--- fresh\n%s", name, got, want)
		}
		if memo.Misses != misses {
			t.Errorf("%s: judged %d declarations, want %d (%d answered)", name, memo.Misses, misses, memo.Hits)
		}
	}
	decls := len(reuseDecls(t, text))
	round("first", text, decls)
	round("no change", text, 0)
	// One table edited: itself, and the next table, whose prev_id
	// relationship names it.
	round("edit one table", strings.Replace(text, "c002 text [not null]", "c002 text [not null, note: 'edited']", 1), 2)
	round("fix it", text, 2)
	// The partial every table injects: every table's columns change.
	round("edit the partial", strings.Replace(text, "created_at timestamp [not null, default: `CURRENT_TIMESTAMP`]", "created_at timestamp [not null]", 1), 7)
	// A rule set that differs judges everything again.
	f, _, _, _ := parser.ParseFileReuse("schema.volt", text, reuse)
	info, _ := check.FileMemo(f, &schema)
	RunMemo(f, info, nil, &memo, ByName("redundantnull"))
	if memo.Misses != decls {
		t.Errorf("other rules: judged %d declarations, want %d", memo.Misses, decls)
	}
}

func reuseDecls(t *testing.T, text string) []ast.Decl {
	t.Helper()
	f, _ := parser.ParseFile("schema.volt", text)
	var out []ast.Decl
	for _, d := range f.Decls {
		if d != nil {
			out = append(out, d)
		}
	}
	return out
}
