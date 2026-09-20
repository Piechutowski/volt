package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
)

// indexShape renders a navigation index for comparison: every
// definition and reference with its span, and every occurrence of the
// per-package index with its position.
func indexShape(ix *voltIndex, pr *lang.Project) string {
	var b strings.Builder
	var defs []string
	for sym, def := range ix.defs {
		defs = append(defs, fmt.Sprintf("def %v %s-%s %d", sym, def.span.pos, def.span.end, len(def.md)))
	}
	sortStrings(defs)
	for _, d := range defs {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	for _, r := range ix.refs {
		fmt.Fprintf(&b, "ref %v %s-%s %s-%s %v %s\n", r.sym, r.span.pos, r.span.end, r.edit.pos, r.edit.end, r.decl, r.text)
	}
	return b.String()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// TestIndexMemoMatchesFreshBuild proves the navigation index built
// through the memo, edit by edit on a one-file project, is the index
// a fresh build produces, and that an edit inside one table rebuilds
// that table's occurrences alone (D85).
func TestIndexMemoMatchesFreshBuild(t *testing.T) {
	root := t.TempDir()
	if err := corpus.Write(root, corpus.Spec{Tables: 12, Columns: 6, Single: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	var s lang.Session
	var memo voltIndexMemo
	round := func(name, text string, wantMisses int) {
		t.Helper()
		overlay := map[string]string{path: text}
		res := projectAnalyze(root, overlay, &s, &memo)
		if res == nil {
			t.Fatalf("%s: no result", name)
		}
		fresh := buildVoltIndex(res.pr, overlay, nil)
		if a, b := indexShape(res.vindex, res.pr), indexShape(fresh, res.pr); a != b {
			t.Fatalf("%s: memoized index differs from a fresh build\n--- memo\n%s--- fresh\n%s", name, a, b)
		}
		im := memo.index["."]
		if im == nil {
			for _, m := range memo.index {
				im = m
			}
		}
		if im.Misses != wantMisses {
			t.Errorf("%s: %d tables rebuilt, want %d (%d reused)", name, im.Misses, wantMisses, im.Hits)
		}
	}
	round("first", text, 12)
	round("no change", text, 0)
	// One table edited rebuilds two: itself, and the next table of
	// the corpus, whose prev_id references it (an input, D86).
	round("edit one table", strings.Replace(text, "c002 text [not null]", "c002 text [not null, note: 'edited']", 1), 2)
	round("edit a route", strings.Replace(text, "get /events        volt.Events", "get /stream        volt.Events", 1), 2)
	round("edit the partial", strings.Replace(text, "created_at timestamp", "created_at timestamp [note: 'stamped']", 1), 12)
	// The enum every table's status column names: its declaration is
	// an input of each table's occurrences (the type reference binds
	// to it), so a change to it rebuilds every table. Another enum,
	// named by no table, rebuilds every table too, one level down: the
	// enum set is an input of every table's check (D86), so the checked
	// tables are new objects and the index follows them.
	cur := strings.Replace(text, "retired [note: 'no longer written']", "retired [note: 'gone']\n\tarchived", 1)
	round("edit the enum", cur, 12)
	cur = strings.Replace(cur, "TablePartial stamped", "Enum kind {\n\tplain\n}\n\nTablePartial stamped", 1)
	round("add another enum", cur, 12)
	// A table whose partial vanishes rebuilds, as does one whose enum does.
	cur = strings.Replace(cur, "TablePartial stamped", "TablePartial stampedx", 1)
	round("rename the partial", cur, 12)
}
