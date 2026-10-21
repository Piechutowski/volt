package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/golang"
)

const selectsCorpus = "rollups"

func TestGoldenSelects(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "testdata", selectsCorpus+".volt"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module corpus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "metrics")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, selectsCorpus+".volt"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("corpus schema must be valid: %v", diags)
	}
	pkg := pr.Packages["metrics"]
	if pkg == nil {
		t.Fatal("package metrics not loaded")
	}
	fns := pkg.SelectFns()
	if len(fns) == 0 {
		t.Fatal("the corpus schema declares no selects")
	}
	out, err := golang.GenerateSelects(pkg.Merged(), pkg.Schema(), fns, golang.Options{Package: "models", Source: selectsCorpus + ".volt"})
	if err != nil {
		t.Fatalf("GenerateSelects: %v", err)
	}
	golang.GoldenCompare(t, out, filepath.Join("testdata", selectsCorpus+"_selects.go.golden"))
}
