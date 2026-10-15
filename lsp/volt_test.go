package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func voltProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestVoltProjectDiagnostics(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":          "module lsptest\n",
		"db/schema.volt":  "package db\n\nTable users {\n\tid integer [pk]\n}\n",
		"app/routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	path := filepath.Join(root, "app", "routes.volt")
	buffer := "package app\n\nimport (\n\tdb\n)\n\nScope / {\n\tget / Home.Index\n}\n"
	d := NewDocument("file://"+path, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.HasPrefix(dg.Code, "spec/imports") && strings.Contains(dg.Msg, "import") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unused import in the buffer not reported; diags = %v", d.Diags)
	}
}

func TestVoltProjectCrossFileConflict(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":     "module lsptest\n",
		"app/a.volt": "package app\n\nScope / {\n\tget /x Home.A\n}\n",
		"app/b.volt": "package app\n\nScope / {\n\tget /y Home.B\n}\n",
	})
	path := filepath.Join(root, "app", "b.volt")
	buffer := "package app\n\nScope / {\n\tget /x Home.B\n}\n"
	d := NewDocument("file://"+path, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "conflict") && dg.Pos.Filename() == path {
			found = true
		}
	}
	if !found {
		t.Fatalf("cross-file route conflict not reported in the open file; diags = %v", d.Diags)
	}
}

func TestVoltFileOutsideProjectKeepsDBML(t *testing.T) {
	clean := NewDocument("file:///lone.volt",
		"package app\n\nScope / {\n\tget / Home.Index\n}\n\nTable users {\n\tid int [pk]\n}\n")
	if len(clean.Diags) != 0 {
		t.Fatalf("projectless .volt file should be quiet, got %v", clean.Diags)
	}

	dup := NewDocument("file:///lone.volt",
		"Table t {\n\tid int [pk]\n}\n\nTable t {\n\tid int [pk]\n}\n")
	if len(dup.Diags) == 0 {
		t.Fatal("duplicate table must still be reported without a project")
	}
}

func TestVoltProjectSkipsForeignFiles(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":          "module lsptest\n",
		"app/routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	dup := "Table t {\n\tid int [pk]\n}\n\nTable t {\n\tid int [pk]\n}\n"

	dbml := NewDocument("file://"+filepath.Join(root, "app", "notes.dbml"), dup)
	if len(dbml.Diags) == 0 {
		t.Error("duplicate table in a .dbml file under a Volt project went unreported")
	}
	draft := NewDocument("file://"+filepath.Join(root, "_drafts", "x.volt"), dup)
	if len(draft.Diags) == 0 {
		t.Error("duplicate table in a skipped-directory .volt file went unreported")
	}
}

func TestVoltProjectEncodedURI(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":             "module lsptest\n",
		"db/schema.volt":     "package db\n\nTable users {\n\tid integer [pk]\n}\n",
		"app/my routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	path := filepath.Join(root, "app", "my routes.volt")
	uri := "file://" + strings.ReplaceAll(path, " ", "%20")
	buffer := "package app\n\nimport (\n\tdb\n)\n\nScope / {\n\tget / Home.Index\n}\n"
	d := NewDocument(uri, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.HasPrefix(dg.Code, "spec/imports") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unused import unreported through an encoded URI; diags = %v", d.Diags)
	}
}

func TestVoltSiblingBuffersOverlay(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":     "module lsptest\n",
		"app/a.volt": "package app\n\nScope / {\n\tget /y Home.A\n}\n",
		"app/b.volt": "package app\n\nScope / {\n\tget /z Home.B\n}\n",
	})
	pathA := filepath.Join(root, "app", "a.volt")
	pathB := filepath.Join(root, "app", "b.volt")
	bufA := "package app\n\nScope / {\n\tget /x Home.A\n}\n"

	b := &Document{URI: "file://" + pathB, Siblings: func() map[string]string {
		return map[string]string{pathA: bufA}
	}}
	b.Update("package app\n\nScope / {\n\tget /x Home.B\n}\n")

	found := false
	for _, dg := range b.Diags {
		if strings.Contains(dg.Msg, "conflict") {
			found = true
		}
	}
	if !found {
		t.Fatalf("conflict with an unsaved sibling buffer unreported; diags = %v", b.Diags)
	}
}

func TestVoltModDiagnosticsSurface(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod":     "modul lsptest\n",
		"app/r.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	d := NewDocument("file://"+filepath.Join(root, "app", "r.volt"),
		"package app\n\nScope / {\n\tget / Home.Index\n}\n")

	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "go.mod") && dg.Pos.Line() == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("broken go.mod surfaced nowhere; diags = %v", d.Diags)
	}
}

func TestVoltSettingsGatedByContext(t *testing.T) {
	for _, src := range []string{
		"Table t {\n\tdelete bool [\n}\n",
		"Table t {\n\tscope varchar [\n}\n",
	} {
		d := doc(t, src)
		pos := posOf(t, src, "[", 0)
		pos.Character++
		labels := map[string]bool{}
		for _, it := range d.Complete(pos) {
			labels[strings.TrimSpace(it.Label)] = true
		}
		if !labels["pk"] || labels["pipe:"] || labels["name:"] && labels["error_handler:"] {
			t.Errorf("column settings hijacked by Volt heuristics for %q: %v", src, labels)
		}
	}
}

func TestDiagnosticSourceIsVolt(t *testing.T) {
	d := doc(t, "Table t {\n\tid int [pk]\n}\n\nTable t {\n\tid int [pk]\n}\n")
	lsp := d.LSPDiagnostics()
	if len(lsp) == 0 {
		t.Fatal("expected diagnostics")
	}
	if lsp[0].Source == nil || *lsp[0].Source != "volt" {
		t.Fatalf("diagnostic source = %v, want volt", lsp[0].Source)
	}
}

func TestVoltCompletionScopeBody(t *testing.T) {
	text := "Scope / {\n\t\n}\n"
	d := doc(t, text)
	items := d.Complete(posOf(t, text, "\t", 0))
	labels := map[string]bool{}
	for _, it := range items {
		labels[strings.TrimSpace(it.Label)] = true
	}
	for _, want := range []string{"get", "post", "resources", "Scope", "any"} {
		if !labels[want] {
			t.Errorf("scope body completion missing %q; got %v", want, labels)
		}
	}
}

func TestVoltCompletionPipelineBody(t *testing.T) {
	text := "Pipeline api {\n\t\n}\n"
	d := doc(t, text)
	items := d.Complete(posOf(t, text, "\t", 0))
	if len(items) != 1 || strings.TrimSpace(items[0].Label) != "use" {
		t.Fatalf("pipeline body completion = %v, want just 'use'", items)
	}
}

func TestVoltCompletionScopeSettings(t *testing.T) {
	text := "Scope / [\n"
	d := doc(t, text)
	pos := posOf(t, text, "[", 0)
	pos.Character++
	items := d.Complete(pos)
	labels := map[string]bool{}
	for _, it := range items {
		labels[strings.TrimSpace(it.Label)] = true
	}
	for _, want := range []string{"pipe:", "error_handler:", "name:"} {
		if !labels[want] {
			t.Errorf("scope settings completion missing %q; got %v", want, labels)
		}
	}
}
