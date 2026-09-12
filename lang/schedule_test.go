package lang_test

// The schedule is not part of the output (PERF-7): loading, checking
// and generating on one CPU and on every CPU yield the same
// diagnostics in the same order, and the same bytes.

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

// scheduleFixture is a project of several data and routing packages,
// each carrying errors of both layers, so the diagnostics come from
// every phase and every package.
func scheduleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(name, src string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module sched\n")
	for i := 0; i < 6; i++ {
		write(fmt.Sprintf("d%d/schema.volt", i), fmt.Sprintf(`package d%d

Table posts {
  id    integer [pk, increment]
  title text    [not null]
  title text
}

Table tags {
  id integer [pk]
}
`, i))
		write(fmt.Sprintf("r%d/routes.volt", i), fmt.Sprintf(`package r%d

import (
	d%d
)

Scope /api {
	resources d%d.posts [default]
	get /nope d%d.PostNope
	get /tags/:id Tags.Show
	get /tags/:id Tags.Also
}
`, i, i, i, i))
	}
	return root
}

// withProcs runs fn with GOMAXPROCS set to n and restores it.
func withProcs(n int, fn func()) {
	prev := runtime.GOMAXPROCS(n)
	defer runtime.GOMAXPROCS(prev)
	fn()
}

func TestCheckScheduleIndependent(t *testing.T) {
	root := scheduleFixture(t)
	run := func() string {
		pr, err := lang.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, d := range lang.Check(pr) {
			b.WriteString(d.String())
			b.WriteByte('\n')
		}
		return b.String()
	}
	var serial string
	withProcs(1, func() { serial = run() })
	if strings.Count(serial, "error") < 12 {
		t.Fatalf("the fixture carries too few errors to prove anything:\n%s", serial)
	}
	for i := 0; i < 3; i++ {
		if got := run(); got != serial {
			t.Fatalf("diagnostics differ between one CPU and %d:\n--- one\n%s--- many\n%s", runtime.GOMAXPROCS(0), serial, got)
		}
	}
}

func TestGenerateScheduleIndependent(t *testing.T) {
	root := perfFixture(t, 12, 8, true)
	run := func() map[string][]byte {
		pr := perfLoad(t, root)
		if diags := lang.Check(pr); diag.HasErrors(diags) {
			t.Fatal(diags)
		}
		out := map[string][]byte{}
		for path, pkg := range pr.Packages {
			if pkg.HasSchema() {
				files, err := model.Generate(pkg, model.Options{Source: "package " + path, SQL: true})
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range files {
					out[path+"/"+f.Name] = f.Code
				}
			}
			if pkg.HasRouting() {
				files, err := router.Generate(pkg, router.Options{Source: "package " + path})
				if err != nil {
					t.Fatal(err)
				}
				for name, code := range files {
					out[path+"/"+name] = code
				}
			}
		}
		return out
	}
	var serial map[string][]byte
	withProcs(1, func() { serial = run() })
	if len(serial) < 8 {
		t.Fatalf("only %d files generated", len(serial))
	}
	for i := 0; i < 3; i++ {
		got := run()
		if len(got) != len(serial) {
			t.Fatalf("%d files on one CPU, %d on %d", len(serial), len(got), runtime.GOMAXPROCS(0))
		}
		for name, code := range serial {
			if string(got[name]) != string(code) {
				t.Fatalf("%s differs between one CPU and %d", name, runtime.GOMAXPROCS(0))
			}
		}
	}
}
