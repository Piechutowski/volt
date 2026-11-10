package golang_test

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Piechutowski/volt/internal/dbtest"
	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	sqlitegen "github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/naming"
	_ "github.com/mattn/go-sqlite3"
)

func TestSelectsPrepare(t *testing.T) {
	root := filepath.Join("testdata", "selects")
	pr, err := lang.LoadDirs(disk.Source{}, root, []string{filepath.Join(root, "db")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	pkg := pr.PackageAt(filepath.Join(root, "db"))
	if pkg == nil {
		t.Fatal("fixture package not loaded")
	}
	ddl, err := sqlitegen.Generate(pkg.Merged(), pkg.Schema(), sqlitegen.Options{Source: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	fns := pkg.SelectFns()
	plan := naming.PlanBuild(pkg.Merged(), pkg.Schema())
	if len(fns) < 4 {
		t.Fatalf("fixture declares too few select instantiations: %d", len(fns))
	}
	type stmtCase struct {
		SQL string
		N   int
	}
	stmts := map[string]stmtCase{}
	for _, fn := range fns {
		sql, args, err := plan.SelectStatement(fn)
		if err != nil {
			t.Fatal(err)
		}
		stmts[fn.TableKey+"."+fn.MethodSuffix] = stmtCase{SQL: sql, N: len(args)}
		if fn.With != nil {
			child, err := plan.WithChildSQL(fn)
			if err != nil {
				t.Fatal(err)
			}
			stmts[fn.TableKey+"."+fn.MethodSuffix+".children"] = stmtCase{SQL: child, N: 1}
		}
	}
	db := dbtest.Open(t, string(ddl))
	for _, name := range slices.Sorted(maps.Keys(stmts)) {
		if err := dbtest.StatementPrepares(db, stmts[name].SQL, stmts[name].N); err != nil {
			t.Errorf("select %s does not prepare against the generated DDL: %v", name, err)
		}
	}
}
