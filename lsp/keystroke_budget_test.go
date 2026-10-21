package lsp

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
)

func allocated(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

var keystrokeBudget = struct {
	document, analysis, publish uint64
}{
	document: 1135112,
	analysis: 4838304,
	publish:  49216,
}

func TestKeystrokeAllocationBudget(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation budget is asserted without the race detector")
	}
	root := t.TempDir()
	spec := corpus.Spec{Tables: 200, Columns: 12, Single: true}
	if err := corpus.Write(root, spec); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "schema.volt")
	text := corpus.Files(spec)["schema.volt"]
	anchor := "c002 string\n"
	if !strings.Contains(text, anchor) {
		t.Fatal("anchor")
	}
	s := &lang.Session{}
	var memo voltIndexMemo
	d := &Document{URI: "file://" + path}
	d.UpdateLocal(text)
	if projectAnalyze(root, map[string]string{path: text}, s, &memo) == nil {
		t.Fatal("the project does not load")
	}
	edited := strings.Replace(text, anchor, "c002 string [note: 'edited']\n", 1)
	overlay := map[string]string{path: edited}
	document := allocated(func() { d.UpdateLocal(edited) })
	var res *projectResult
	analysis := allocated(func() { res = projectAnalyze(root, overlay, s, &memo) })
	if res == nil {
		t.Fatal("the edited project does not load")
	}
	publish := allocated(func() { diagnosticsLSP(edited, lineStarts(edited), res.docDiags(path)) })
	for _, part := range []struct {
		name        string
		got, budget uint64
	}{{"document", document, keystrokeBudget.document}, {"analysis", analysis, keystrokeBudget.analysis}, {"publish", publish, keystrokeBudget.publish}} {
		t.Logf("%-9s %10d B (budget %d B)", part.name, part.got, part.budget)
		if part.budget == 0 {
			t.Errorf("%s: no budget set; set keystrokeBudget.%s from the measurement above", part.name, part.name)
			continue
		}
		if part.got > part.budget+part.budget/10 {
			t.Errorf("%s allocates %d B, over its budget of %d B by more than ten percent", part.name, part.got, part.budget)
		}
	}
}
