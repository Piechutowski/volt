package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
)

// TestGoScanReusedAcrossBuilds proves the navigation index keeps a
// package directory's Go scan across builds while the sources are the
// bytes they were, and scans again when they are not (D105, D87).
func TestGoScanReusedAcrossBuilds(t *testing.T) {
	root := t.TempDir()
	if err := corpus.Write(root, corpus.Spec{Tables: 4, Columns: 4, Single: true}); err != nil {
		t.Fatal(err)
	}
	var s lang.Session
	var memo voltIndexMemo
	scan := func(name string) string {
		t.Helper()
		res := projectAnalyze(root, nil, &s, &memo)
		if res == nil {
			t.Fatalf("%s: no result", name)
		}
		if res.vindex.goStale() {
			t.Fatalf("%s: the index reads as stale right after its build", name)
		}
		e, ok := memo.goscans[root]
		if !ok || len(res.vindex.gofuncs) == 0 {
			t.Fatalf("%s: no Go scan kept for the package directory (%d scanned)", name, len(res.vindex.gofuncs))
		}
		return fmt.Sprintf("%p", e.funcs)
	}
	first := scan("first")
	if again := scan("no change"); again != first {
		t.Fatal("the Go scan was redone for unchanged sources")
	}
	if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package main\n\nfunc Extra() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := scan("a Go file added"); changed == first {
		t.Fatal("the Go scan was kept although the sources changed")
	}
}
