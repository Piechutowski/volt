package golang_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/nao/gen/golang"
	sqlitegen "github.com/Piechutowski/volt/nao/gen/sqlite"
)

// TestSelectsPrepare is the §V11.6 promise made executable: every select
// statement the generator emits for the fixture package — group
// selects, projections, typed checks in the DDL — must prepare on a
// real SQLite loaded with the DDL gen/sqlite emits from the same schema
// (D06). EXPLAIN compiles without running; named parameters bind NULL.
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
	if len(fns) < 4 {
		t.Fatalf("fixture declares too few select instantiations: %d", len(fns))
	}
	stmts := map[string]string{}
	for _, fn := range fns {
		sql, err := golang.SelectSQLFor(pkg.Merged(), pkg.Schema(), fn)
		if err != nil {
			t.Fatal(err)
		}
		stmts[fn.TableKey+"."+fn.MethodSuffix] = sql
	}
	input, err := json.Marshal(map[string]any{"ddl": string(ddl), "stmts": stmts})
	if err != nil {
		t.Fatal(err)
	}
	const driver = `
import sqlite3, sys, json, collections
inp = json.load(sys.stdin)
con = sqlite3.connect(":memory:")
con.execute("PRAGMA foreign_keys = ON")
con.executescript(inp["ddl"])
named = collections.defaultdict(lambda: None)
for name, stmt in sorted(inp["stmts"].items()):
    try:
        con.execute("EXPLAIN " + stmt, named)
    except Exception as e:
        sys.exit(f"{name}: {e}\n  {stmt}")
`
	cmd := exec.Command("python3", "-c", driver)
	cmd.Stdin = strings.NewReader(string(input))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("a select does not prepare against the generated DDL: %v\n%s", err, out)
	}
}
