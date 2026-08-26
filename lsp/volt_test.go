package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// voltProject materializes files on disk (a real project for FindRoot
// and LoadOverlay to see) and returns its root.
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

// TestVoltProjectDiagnostics: a .volt file under a volt.mod root is
// checked as part of the whole project — the open buffer overlays the
// saved file, so the unused import typed into the buffer is reported
// even though the disk copy is clean.
func TestVoltProjectDiagnostics(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":        "module lsptest\n",
		"db/schema.volt":  "package db\n\nTable users {\n\tid integer [pk]\n}\n",
		"app/routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	path := filepath.Join(root, "app", "routes.volt")
	buffer := "package app\n\nimport (\n\tdb\n)\n\nScope / {\n\tget / Home.Index\n}\n"
	d := NewDocument("file://"+path, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.HasPrefix(dg.Code, "spec/V2") && strings.Contains(dg.Msg, "import") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unused import in the buffer not reported; diags = %v", d.Diags)
	}
}

// TestVoltProjectCrossFileConflict: the project view spans package
// files, so a route conflicting with one declared in a sibling file is
// caught — the single-file pass could never see it.
func TestVoltProjectCrossFileConflict(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":   "module lsptest\n",
		"app/a.volt": "package app\n\nScope / {\n\tget /x Home.A\n}\n",
		"app/b.volt": "package app\n\nScope / {\n\tget /y Home.B\n}\n",
	})
	path := filepath.Join(root, "app", "b.volt")
	buffer := "package app\n\nScope / {\n\tget /x Home.B\n}\n"
	d := NewDocument("file://"+path, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "conflict") && dg.Pos.Filename == path {
			found = true
		}
	}
	if !found {
		t.Fatalf("cross-file route conflict not reported in the open file; diags = %v", d.Diags)
	}
}

// TestVoltFileOutsideProjectKeepsDBML: with no volt.mod above it, a
// .volt file gets the single-file DBML pass — Volt declarations parse
// (superset, §V0) without bogus squiggles, and DBML errors still show.
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

// TestVoltProjectSkipsForeignFiles: the project pass owns only files
// the loader actually read. A .dbml file under a volt.mod root, or a
// .volt file in a skipped directory (§V1.6), keeps the single-file
// pass — its diagnostics must not silently vanish.
func TestVoltProjectSkipsForeignFiles(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":        "module lsptest\n",
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

// TestVoltProjectEncodedURI: editors percent-encode file URIs; the
// decoded path must still hit the overlay and the diagnostic filter.
func TestVoltProjectEncodedURI(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":           "module lsptest\n",
		"db/schema.volt":     "package db\n\nTable users {\n\tid integer [pk]\n}\n",
		"app/my routes.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	path := filepath.Join(root, "app", "my routes.volt")
	uri := "file://" + strings.ReplaceAll(path, " ", "%20")
	buffer := "package app\n\nimport (\n\tdb\n)\n\nScope / {\n\tget / Home.Index\n}\n"
	d := NewDocument(uri, buffer)

	found := false
	for _, dg := range d.Diags {
		if strings.HasPrefix(dg.Code, "spec/V2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unused import unreported through an encoded URI; diags = %v", d.Diags)
	}
}

// TestVoltSiblingBuffersOverlay: cross-file analysis sees other OPEN
// buffers, not their stale saved copies — a conflict typed into an
// unsaved sibling shows up here.
func TestVoltSiblingBuffersOverlay(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":   "module lsptest\n",
		"app/a.volt": "package app\n\nScope / {\n\tget /y Home.A\n}\n",
		"app/b.volt": "package app\n\nScope / {\n\tget /z Home.B\n}\n",
	})
	pathA := filepath.Join(root, "app", "a.volt")
	pathB := filepath.Join(root, "app", "b.volt")
	bufA := "package app\n\nScope / {\n\tget /x Home.A\n}\n" // unsaved edit

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

// TestVoltModDiagnosticsSurface: volt.mod is never open in the editor,
// so its problems attach to the top of every project file.
func TestVoltModDiagnosticsSurface(t *testing.T) {
	root := voltProject(t, map[string]string{
		"volt.mod":   "modul lsptest\n",
		"app/r.volt": "package app\n\nScope / {\n\tget / Home.Index\n}\n",
	})
	d := NewDocument("file://"+filepath.Join(root, "app", "r.volt"),
		"package app\n\nScope / {\n\tget / Home.Index\n}\n")

	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "volt.mod") && dg.Pos.Line == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("broken volt.mod surfaced nowhere; diags = %v", d.Diags)
	}
}

// TestVoltSettingsGatedByContext: a Table column named like a verb or
// 'scope' still gets column settings — the Volt settings heuristics
// only fire where routes can exist (§V4).
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

// TestDiagnosticSourceIsVolt: editors label diagnostics by their
// Source field — the language's name, whichever binary serves it.
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

// TestVoltCompletionScopeBody: inside a Scope body the verbs,
// 'resources' and nested 'Scope' complete.
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

// TestVoltCompletionPipelineBody: a Pipeline body offers 'use'.
func TestVoltCompletionPipelineBody(t *testing.T) {
	text := "Pipeline api {\n\t\n}\n"
	d := doc(t, text)
	items := d.Complete(posOf(t, text, "\t", 0))
	if len(items) != 1 || strings.TrimSpace(items[0].Label) != "use" {
		t.Fatalf("pipeline body completion = %v, want just 'use'", items)
	}
}

// TestVoltCompletionScopeSettings: '[' on a Scope line offers the scope
// settings; on a route line, the route settings.
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
