package itest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/nao/edbml/diag"
)

// TestFixtureNotDrifted regenerates the fixture's router files
// in-memory and byte-compares them with the committed ones: the proof
// suite always exercises current generator output. Refresh with
// 'go run ./cmd/volt gen ./itest/fadn' after intentional changes.
func TestFixtureNotDrifted(t *testing.T) {
	pr, err := lang.Load("fadn")
	if err != nil {
		t.Fatal(err)
	}
	diags := lang.Check(pr)
	if diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	files, err := router.Generate(pr.Packages["app"], router.Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("fadn", "app", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s drifted from generator output; re-run volt gen ./itest/fadn", name)
		}
	}
}
