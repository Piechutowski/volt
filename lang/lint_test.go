package lang_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestVetRunsAnalyzers pins docs/lint.md's promise: `volt vet` runs
// every analyzer over a project's packages, beside the project rule.
func TestVetRunsAnalyzers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module vetted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	// missingpk on posts; the pipeline rule on a pipeline nothing pipes through
	src := "package db\n\nTable posts {\n  title text [not null]\n}\n\nPipeline api {\n\tuse volt.RequestID\n}\n\nScope / {\n\tget /x Home.Index\n}\n"
	if err := os.WriteFile(filepath.Join(root, "db", "schema.volt"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	var codes []string
	for _, d := range lang.Vet(pr) {
		codes = append(codes, d.Code)
	}
	got := strings.Join(codes, " ")
	for _, want := range []string{"vet/missingpk", "vet/pipeline"} {
		if !strings.Contains(got, want) {
			t.Errorf("vet did not report %s; got %q", want, got)
		}
	}
}
