package lang_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

func TestVetRunsAnalyzers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module vetted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package db\n\ntable posts {\n  title string\n}\n\npipeline api {\n\tuse volt.RequestID\n}\n\nscope / {\n\tget /x Home.Index\n}\n"
	if err := os.WriteFile(filepath.Join(root, "db", "schema.volt"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	pr, err := lang.Load(disk.Source{}, root)
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

// TestVetTellsTheBodyByItsPlace proves loadspelledlong learns from the
// routes where each action receives its body, so an update of the body
// under any name passes, while a lookup by a path parameter still warns
// (lint.md, loadspelledlong; spec §request-bodies).
func TestVetTellsTheBodyByItsPlace(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":          "module vetted\n",
		"db/schema.volt":  "package db\n\ntable metrics {\n  id   int    [pk]\n  unit string\n}\n",
		"app/routes.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tget   /metrics/:id(int) Metrics.Show\n\tpatch /metrics/:id(int) Metrics.Save [body: db.MetricUpdate(id)]\n\tpost  /metrics          Metrics.Add  [body: db.MetricCreate]\n}\n",
		"app/metrics.go": `package app

import (
	"github.com/Piechutowski/volt"
	"vetted/db"
)

type Metrics struct{}

func (Metrics) Show(r volt.Request, id int) volt.Response {
	return volt.Then(db.MetricGet(id), func(m db.Metric) volt.Response { return volt.Render(m) })
}

func (Metrics) Save(r volt.Request, id int, cs db.MetricChangeset) volt.Response {
	return volt.Then(db.MetricUpdate(cs), func(m db.Metric) volt.Response { return volt.Render(m) })
}

func (Metrics) Add(r volt.Request, in db.MetricChangeset) volt.Response {
	return volt.Then(db.MetricCreate(in), func(m db.Metric) volt.Response { return volt.Render(m) })
}
`,
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr, err := lang.Load(disk.Source{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if diags := lang.Check(pr); diag.HasErrors(diags) {
		t.Fatal(diags)
	}
	var lines []int
	for _, d := range lang.Vet(pr) {
		if d.Code == "vet/loadspelledlong" {
			lines = append(lines, int(d.Pos.Line()))
		}
	}
	if len(lines) != 1 || lines[0] != 11 {
		t.Errorf("loadspelledlong warned at lines %v, want only Show's lookup at line 11", lines)
	}
}
