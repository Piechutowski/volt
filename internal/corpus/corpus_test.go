package corpus_test

import (
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestCorpusChecks proves both layouts check clean at a small size and
// exercise what they claim: selects, checks, pipelines, routes.
func TestCorpusChecks(t *testing.T) {
	for _, spec := range []corpus.Spec{{Packages: 2, Tables: 3, Columns: 9}, {Packages: 1, Tables: 3, Columns: 9, Single: true}} {
		root := t.TempDir()
		if err := corpus.Write(root, spec); err != nil {
			t.Fatal(err)
		}
		pr, err := lang.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		diags := lang.Check(pr)
		if diag.HasErrors(diags) {
			t.Fatalf("%+v does not check:\n%v", spec, diags)
		}
		for path, pkg := range pr.Packages {
			if pkg.HasSchema() && (len(pkg.Selects) != spec.Tables+1 || len(pkg.CheckFns) != spec.Tables) {
				t.Errorf("%+v %s: %d selects, %d check tables", spec, path, len(pkg.Selects), len(pkg.CheckFns))
			}
			if pkg.HasRouting() && (len(pkg.Pipelines) != 1 || len(pkg.Routes) < 5*spec.Tables) {
				t.Errorf("%+v %s: %d pipelines, %d routes", spec, path, len(pkg.Pipelines), len(pkg.Routes))
			}
		}
	}
}
