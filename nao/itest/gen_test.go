package itest

// The generated siblings are checked in so the package always compiles;
// this directive refreshes them after editing schema.volt:
//
//go:generate go run ../../cmd/volt gen --sql .

import (
	"bytes"
	"os"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestGeneratedFilesCurrent proves the checked-in files match what the
// generator produces from schema.volt today — through the same path
// `volt gen` takes, so the test and the tool cannot disagree.
// On failure: go generate ./nao/itest
func TestGeneratedFilesCurrent(t *testing.T) {
	pr, err := lang.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("schema.volt must be valid: %v", diags)
	}
	pkg := pr.Packages["."]
	if pkg == nil || !pkg.HasSchema() {
		t.Fatal("the fixture package declares no data elements")
	}

	files, err := model.Generate(pkg, model.Options{Source: "package " + pkg.Path, SQL: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		got, err := os.ReadFile(f.Name)
		if err != nil {
			t.Errorf("%s: %v (run 'go generate ./nao/itest')", f.Name, err)
			continue
		}
		if !bytes.Equal(got, f.Code) {
			t.Errorf("%s is stale; run 'go generate ./nao/itest'", f.Name)
		}
	}
}
