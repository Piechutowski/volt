package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestExampleNotDrifted keeps the committed volt_*.go files honest:
// they must be exactly what today's generator emits from routes.volt.
// Refresh with 'go run ./cmd/volt gen ./examples/crud'.
func TestExampleNotDrifted(t *testing.T) {
	pr, err := lang.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("example project has errors: %v", diags)
	}
	files, err := router.Generate(pr.Packages["app"], router.Options{Source: "package app"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("app", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s drifted; re-run volt gen ./examples/crud", name)
		}
	}
}
