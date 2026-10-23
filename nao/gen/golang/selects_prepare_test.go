package golang_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	sqlitegen "github.com/Piechutowski/volt/nao/gen/sqlite"
	"github.com/Piechutowski/volt/nao/naming"
)

func TestSelectsPrepare(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	root := filepath.Join("testdata", "selects")
	pr, err := lang.LoadDirs(root, []string{filepath.Join(root, "db")}, nil)
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
		SQL string `json:"sql"`
		N   int    `json:"n"`
	}
	stmts := map[string]stmtCase{}
	for _, fn := range fns {
		sql, args, err := plan.SelectStatement(fn)
		if err != nil {
			t.Fatal(err)
		}
		stmts[fn.TableKey+"."+fn.MethodSuffix] = stmtCase{SQL: sql, N: len(args)}
	}
	input, err := json.Marshal(map[string]any{"ddl": string(ddl), "stmts": stmts})
	if err != nil {
		t.Fatal(err)
	}
	const driver = `
import sqlite3, sys, json
inp = json.load(sys.stdin)
con = sqlite3.connect(":memory:")
con.execute("PRAGMA foreign_keys = ON")
con.executescript(inp["ddl"])
for name, case in sorted(inp["stmts"].items()):
    stmt, n = case["sql"], case["n"]
    try:
        con.execute("EXPLAIN " + stmt, [None] * n)
    except Exception as e:
        sys.exit(f"{name}: {e}\n  {stmt}")
`
	cmd := exec.Command("python3", "-c", driver)
	cmd.Stdin = strings.NewReader(string(input))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("a select does not prepare against the generated DDL: %v\n%s", err, out)
	}
}
