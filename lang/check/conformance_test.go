package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestConformanceCorpus runs the schema half of the spec's snippet
// corpus (the .dbml entries of lang/conformance/snippets) through the
// single-file front end: every valid snippet must produce zero errors,
// every invalid snippet at least one. The verdicts were pinned against
// the upstream @dbml/parse compiler while the cross-check existed
// (retired at 0 disagreements, D54).
func TestConformanceCorpus(t *testing.T) {
	root := filepath.Join("..", "conformance", "snippets")
	for _, group := range []struct {
		dir       string
		wantError bool
	}{
		{"valid", false},
		{"invalid", true},
	} {
		files, err := filepath.Glob(filepath.Join(root, group.dir, "*.dbml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no snippets in %s", filepath.Join(root, group.dir))
		}
		for _, file := range files {
			file := file
			t.Run(group.dir+"/"+filepath.Base(file), func(t *testing.T) {
				src, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				f, diags := parser.ParseFile(file, string(src))
				_, semDiags := check.File(f)
				diags = append(diags, semDiags...)
				gotError := diag.HasErrors(diags)
				if gotError != group.wantError {
					t.Errorf("want error=%v, got error=%v; diagnostics: %v", group.wantError, gotError, diags)
				}
			})
		}
	}
}
