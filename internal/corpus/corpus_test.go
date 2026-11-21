package corpus_test

import (
	"testing"

	"github.com/Piechutowski/volt/internal/corpus"
	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

func TestCorpusChecks(t *testing.T) {
	for _, spec := range []corpus.Spec{{Packages: 2, Tables: 3, Columns: 9}, {Packages: 1, Tables: 3, Columns: 9, Single: true}} {
		root := t.TempDir()
		if err := corpus.Write(root, spec); err != nil {
			t.Fatal(err)
		}
		pr, err := lang.Load(disk.Source{}, root)
		if err != nil {
			t.Fatal(err)
		}
		diags := lang.Check(pr)
		if diag.HasErrors(diags) {
			t.Fatalf("%+v does not check:\n%v", spec, diags)
		}
		for path, pkg := range pr.Packages {
			if pkg.HasSchema() && (len(pkg.Selects) != spec.Tables+1 || len(pkg.Validators) != spec.Tables) {
				t.Errorf("%+v %s: %d selects, %d check tables", spec, path, len(pkg.Selects), len(pkg.Validators))
			}
			if pkg.HasRouting() && (len(pkg.Pipelines) != 1 || len(pkg.Routes) < 5*spec.Tables) {
				t.Errorf("%+v %s: %d pipelines, %d routes", spec, path, len(pkg.Pipelines), len(pkg.Routes))
			}
		}
	}
}

func TestTablesHoldColumnsBeyondDataAtMost(t *testing.T) {
	spec := corpus.Spec{Tables: 3, Columns: 9, Single: true}
	file, diags := parser.ParseFile("schema.volt", corpus.Files(spec)["schema.volt"])
	if diag.HasErrors(diags) {
		t.Fatalf("the stress schema does not parse:\n%v", diags)
	}
	info, diags := check.File(file)
	if diag.HasErrors(diags) {
		t.Fatalf("the stress schema does not check:\n%v", diags)
	}
	widest := 0
	for _, table := range info.Tables {
		widest = max(widest, len(table.Columns))
	}
	if widest != spec.Columns+corpus.ColumnsBeyondData {
		t.Fatalf("the widest table has %d columns, want %d data columns and %d beyond them",
			widest, spec.Columns, corpus.ColumnsBeyondData)
	}
}
