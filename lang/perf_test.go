package lang_test

// Scaling tests (roadmap PERF-2): the work a phase does must grow
// linearly with its input. Wall-clock cannot be asserted on shared
// hardware, but allocation counts can — they are exact for a given
// toolchain, so a doubling of tables that more than triples the bytes
// allocated is an algorithmic regression, whatever machine ran it. The
// cubic path D74 removed would have failed these at eight times.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/gen/model"
	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// perfFixture writes a project with one data package of the given
// tables and columns and one routing package that declares a resources
// per table — with [default], the shape that binds every action to the
// table's generated CRUD; without, the controller shape that only
// exercises route expansion and conflict detection.
func perfFixture(t *testing.T, tables, cols int, def bool) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module perf\n\ngo 1.27\n")

	var db strings.Builder
	db.WriteString("package db\n\n")
	for i := 1; i <= tables; i++ {
		fmt.Fprintf(&db, "Table m_t%04ds {\n\tid integer [pk, increment]\n", i)
		for c := 1; c < cols; c++ {
			typ := "text"
			if c%2 == 1 {
				typ = "integer"
			}
			fmt.Fprintf(&db, "\tc%03d %s [not null]\n", c, typ)
		}
		db.WriteString("}\n\n")
	}
	write("db/schema.volt", db.String())

	var app strings.Builder
	app.WriteString("package app\n\nimport (\n\tdb\n)\n\nScope / {\n")
	for i := 1; i <= tables; i++ {
		if def {
			fmt.Fprintf(&app, "\tresources db.m_t%04ds [default]\n", i)
		} else {
			fmt.Fprintf(&app, "\tresources db.m_t%04ds\n", i)
		}
	}
	app.WriteString("}\n")
	write("app/routes.volt", app.String())
	return root
}

// perfLoad loads and checks a fixture, failing on any diagnostic.
func perfLoad(t *testing.T, root string) *lang.Project {
	t.Helper()
	dirs, err := lang.PackageDirs(root, root)
	if err != nil {
		t.Fatal(err)
	}
	pr, err := lang.LoadDirs(root, dirs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatalf("fixture does not check: %v", diags)
	}
	return pr
}

// allocated measures the bytes fn allocates.
func allocated(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// linear asserts that doubling the input did not more than triple the
// allocation: linear growth is 2x, and the slack covers map growth.
func linear(t *testing.T, what string, small, large uint64) {
	t.Helper()
	ratio := float64(large) / float64(small)
	t.Logf("%s: %d B -> %d B (x%.2f)", what, small, large, ratio)
	if ratio > 3 {
		t.Errorf("%s allocates x%.2f for x2 input: super-linear", what, ratio)
	}
}

func TestCheckScalesLinearlyInDefaultResources(t *testing.T) {
	// 5·R·T²·C name derivations before D74: doubling T octupled this.
	measure := func(tables int) uint64 {
		root := perfFixture(t, tables, 24, true)
		dirs, err := lang.PackageDirs(root, root)
		if err != nil {
			t.Fatal(err)
		}
		pr, err := lang.LoadDirs(root, dirs, nil)
		if err != nil {
			t.Fatal(err)
		}
		return allocated(func() {
			if diags := lang.Check(pr); diag.HasErrors(diags) {
				t.Fatalf("fixture does not check: %v", diags)
			}
		})
	}
	linear(t, "check with [default] resources", measure(16), measure(32))
}

func TestCheckScalesLinearlyInRoutes(t *testing.T) {
	// R²/2 pattern comparisons, each re-parsing both patterns, before the
	// route index: doubling the routes quadrupled this.
	measure := func(tables int) uint64 {
		root := perfFixture(t, tables, 4, false)
		dirs, err := lang.PackageDirs(root, root)
		if err != nil {
			t.Fatal(err)
		}
		pr, err := lang.LoadDirs(root, dirs, nil)
		if err != nil {
			t.Fatal(err)
		}
		return allocated(func() {
			if diags := lang.Check(pr); diag.HasErrors(diags) {
				t.Fatalf("fixture does not check: %v", diags)
			}
		})
	}
	linear(t, "check with controller resources", measure(64), measure(128))
}

func TestGenerateScalesLinearlyInTables(t *testing.T) {
	measure := func(tables int) uint64 {
		pr := perfLoad(t, perfFixture(t, tables, 24, true))
		return allocated(func() {
			for _, pkg := range pr.Packages {
				if pkg.HasSchema() {
					if _, err := model.Generate(pkg, model.Options{Source: "perf", SQL: true}); err != nil {
						t.Fatal(err)
					}
				}
				if pkg.HasRouting() {
					if _, err := router.Generate(pkg, router.Options{Source: "perf"}); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
	linear(t, "generate", measure(16), measure(32))
}
