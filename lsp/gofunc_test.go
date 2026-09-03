// Go-reference navigation (§V3.2, §V12.5): gd on a Go-reference check
// or a pipeline plug lands on the function in the package's Go file,
// and hover shows its signature and doc comment.
package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const goRefSchema = "package db\n\nTable users {\n\tid integer [pk, increment]\n\temail varchar [not null]\n\n\tchecks {\n\t\tEmailValid(email)\n\t\tdb.Missing(email)\n\t}\n}\n"
const goRefRoutes = "package app\n\nPipeline api {\n\tuse volt.RequestID\n\tuse Auth\n}\n"

func goRefProject(t *testing.T) string {
	t.Helper()
	return voltProject(t, map[string]string{
		"go.mod":          "module goref\n",
		"db/schema.volt":  goRefSchema,
		"db/checks.go":    "package db\n\nimport \"errors\"\n\n// EmailValid rejects addresses without an @ — a Go-reference check\n// target (spec §V12.5).\nfunc EmailValid(email string) error {\n\tif email == \"\" {\n\t\treturn errors.New(\"empty\")\n\t}\n\treturn nil\n}\n",
		"app/routes.volt": goRefRoutes,
		"app/mw.go":       "package app\n\nimport \"net/http\"\n\n// Auth is a pipeline plug.\nfunc Auth(next http.Handler) http.Handler { return next }\n",
	})
}

func TestGoRefDefinitionAndHover(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	d := NewDocument("file://"+schema, goRefSchema)

	loc := d.Definition(posOf(t, goRefSchema, "EmailValid", 0))
	if loc == nil {
		t.Fatal("no definition for EmailValid")
	}
	if !strings.HasSuffix(loc.URI, "db/checks.go") {
		t.Errorf("definition URI = %s, want db/checks.go", loc.URI)
	}
	// "func EmailValid" is on line 7 (0-based 6); the name starts at col 5.
	if loc.Range.Start.Line != 6 || loc.Range.Start.Character != 5 {
		t.Errorf("definition range = %+v, want line 6 char 5", loc.Range.Start)
	}

	h := d.Hover(posOf(t, goRefSchema, "EmailValid", 0))
	if h == nil {
		t.Fatal("no hover for EmailValid")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"func EmailValid(email string) error", "rejects addresses without an @"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
}

func TestGoRefMissingFunctionHoverExplains(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	d := NewDocument("file://"+schema, goRefSchema)

	if loc := d.Definition(posOf(t, goRefSchema, "Missing", 0)); loc != nil {
		t.Errorf("definition for an undeclared function = %v, want none", loc)
	}
	h := d.Hover(posOf(t, goRefSchema, "Missing", 0))
	if h == nil {
		t.Fatal("no hover for a missing Go reference")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "no such function") {
		t.Errorf("hover should explain the function is undeclared:\n%s", md)
	}
}

func TestGoRefPipelinePlug(t *testing.T) {
	root := goRefProject(t)
	routes := filepath.Join(root, "app", "routes.volt")
	d := NewDocument("file://"+routes, goRefRoutes)

	loc := d.Definition(posOf(t, goRefRoutes, "Auth", 0))
	if loc == nil || !strings.HasSuffix(loc.URI, "app/mw.go") {
		t.Fatalf("plug definition = %v, want app/mw.go", loc)
	}
	h := d.Hover(posOf(t, goRefRoutes, "Auth", 0))
	if h == nil || !strings.Contains(h.Contents.(protocol.MarkupContent).Value, "func Auth(next http.Handler) http.Handler") {
		t.Errorf("plug hover missing the signature: %v", h)
	}
	// volt.RequestID lives in the runtime, not the package: no claim made.
	if loc := d.Definition(posOf(t, goRefRoutes, "RequestID", 0)); loc != nil {
		t.Errorf("volt.RequestID resolved to %v; runtime plugs are not package functions", loc)
	}
}

// TestGoRefUndeclaredIsDiagnostic: a Go reference to a function the
// package's Go files do not declare is an error at the reference
// (§V12.5, D63) — the typo shows up in the editor, and an unwritten
// function reads as a to-do with its exact signature.
func TestGoRefUndeclaredIsDiagnostic(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	d := NewDocument("file://"+schema, goRefSchema)
	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "no function Missing") && strings.Contains(dg.Msg, "func Missing(string) error") {
			found = true
		}
	}
	if !found {
		t.Errorf("undeclared Go reference produced no diagnostic naming the signature to write; diags = %v", d.Diags)
	}
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "EmailValid") {
			t.Errorf("the declared, well-typed reference must not be flagged: %v", dg)
		}
	}
}

// TestGoRefRenameRewritesVoltSpellings: renaming a Go reference in a
// .volt file rewrites the Volt spellings only; the Go declaration is
// gopls' job and the existence error then points at it.
func TestGoRefRenameRewritesVoltSpellings(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	d := NewDocument("file://"+schema, goRefSchema)

	edit, err := d.Rename(posOf(t, goRefSchema, "EmailValid", 0), "AddressValid")
	if err != nil {
		t.Fatal(err)
	}
	var voltEdits, goEdits int
	for uri, edits := range edit.Changes {
		if strings.HasSuffix(uri, ".go") {
			goEdits += len(edits)
		} else {
			voltEdits += len(edits)
		}
	}
	if voltEdits != 1 || goEdits != 0 {
		t.Errorf("rename touched %d volt and %d go spellings, want 1 and 0: %+v", voltEdits, goEdits, edit.Changes)
	}
}

// TestColumnRenameFollowsIntoChecks: renaming a column rewrites its
// uses inside the table's checks — typed operands and Go-reference
// arguments alike.
func TestColumnRenameFollowsIntoChecks(t *testing.T) {
	const text = "package db\n\nTable users {\n\tid integer [pk, increment]\n\temail varchar [not null]\n\n\tchecks {\n\t\temail like '%@%'\n\t\tEmailValid(email)\n\t}\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module rn\n",
		"db/schema.volt": text,
		"db/checks.go":   "package db\n\nfunc EmailValid(email string) error { return nil }\n",
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
	edit, err := d.Rename(posOf(t, text, "email", 0), "address")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	// declaration + like operand + Go-reference argument
	if n != 3 {
		t.Errorf("column rename produced %d edits, want 3 (declaration, check operand, check argument): %+v", n, edit.Changes)
	}
}

// TestColumnRenameFollowsIntoSingleTableSelect: a select over one table
// binds its projection, order and where columns to that table, so a
// column rename rewrites them; a group-targeted select is left alone.
func TestColumnRenameFollowsIntoSingleTableSelect(t *testing.T) {
	const text = "package db\n\nTable page_views {\n\tid integer [pk, increment]\n\tsite varchar [not null]\n\tday integer [not null]\n}\n\n" +
		"Select rows (site, day) for page_views where site = :s [order: (day desc, site asc)]\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module rn\n",
		"db/schema.volt": text,
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
	edit, err := d.Rename(posOf(t, text, "site", 0), "host")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	// declaration + projection + where + order
	if n != 4 {
		t.Errorf("column rename produced %d edits, want 4 (declaration, projection, where, order): %+v", n, edit.Changes)
	}
}

// TestGoRefQualifiedRenameKeepsQualifier: renaming `db.EmailValid`
// rewrites the name only; the package qualifier stays.
func TestGoRefQualifiedRenameKeepsQualifier(t *testing.T) {
	const text = "package db\n\nTable users {\n\tid integer [pk]\n\temail varchar [not null]\n\n\tchecks {\n\t\tdb.EmailValid(email)\n\t}\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module q\n",
		"db/schema.volt": text,
		"db/checks.go":   "package db\n\nfunc EmailValid(email string) error { return nil }\n",
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
	edit, err := d.Rename(posOf(t, text, "EmailValid", 0), "AddressValid")
	if err != nil {
		t.Fatal(err)
	}
	for _, edits := range edit.Changes {
		for _, e := range edits {
			// The edit starts after "db." — column 5 on the check line.
			if e.Range.Start.Character != 5 {
				t.Errorf("rename edit starts at char %d, want 5 (the name, not the qualifier)", e.Range.Start.Character)
			}
		}
	}
}

// TestSelectColumnRefsRespectGroupShadowing: a select targeting a
// Group that shares a table's name binds no column refs (the checker
// resolves groups first, §V11.2), while a schema-qualified table
// targeted by its base name does bind.
func TestSelectColumnRefsRespectGroupShadowing(t *testing.T) {
	const text = "package db\n\nTable core.orders {\n\tid integer [pk]\n\ttotal integer [not null]\n}\n\n" +
		"Table page_views {\n\tid integer [pk]\n\tsite varchar [not null]\n}\n\n" +
		"Table link_clicks {\n\tid integer [pk]\n\tsite varchar [not null]\n}\n\n" +
		"Group page_views {\n\tlink_clicks\n}\n\n" +
		"Select big (total) for orders where total > 1\n" +
		"Select all (site) for page_views\n"
	root := voltProject(t, map[string]string{"go.mod": "module g\n", "db/schema.volt": text})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)

	// core.orders.total: declaration + projection + where = 3 edits.
	edit, err := d.Rename(posOf(t, text, "total", 0), "amount")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n != 3 {
		t.Errorf("schema-qualified table: rename produced %d edits, want 3: %+v", n, edit.Changes)
	}

	// page_views.site: the select targets the GROUP page_views, so only
	// the declaration is renamed.
	edit, err = d.Rename(posOf(t, text, "site", 0), "host")
	if err != nil {
		t.Fatal(err)
	}
	n = 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n != 1 {
		t.Errorf("group-shadowed select: rename produced %d edits, want 1 (declaration only): %+v", n, edit.Changes)
	}
}

// TestPredRefInCheckNavigates: a Pred named inside a checks block has a
// definition, and renaming the Pred follows into the check.
func TestPredRefInCheckNavigates(t *testing.T) {
	const text = "package db\n\nPred positive { hits >= 0 }\n\nTable page_views {\n\tid integer [pk]\n\thits integer [not null]\n\n\tchecks {\n\t\tpositive\n\t}\n}\n"
	root := voltProject(t, map[string]string{"go.mod": "module p\n", "db/schema.volt": text})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
	if loc := d.Definition(posOf(t, text, "positive", 1)); loc == nil || loc.Range.Start.Line != 2 {
		t.Errorf("Pred reference inside checks did not resolve to the declaration: %v", loc)
	}
	edit, err := d.Rename(posOf(t, text, "positive", 0), "nonneg")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n != 2 {
		t.Errorf("Pred rename produced %d edits, want 2 (declaration + check use): %+v", n, edit.Changes)
	}
}

// TestGoFileChangeInvalidatesIndex: after the Go function is renamed on
// disk (gopls' job), the document knows its facts are stale; a re-run
// then drops the definition and reports the reference as undeclared.
func TestGoFileChangeInvalidatesIndex(t *testing.T) {
	root := goRefProject(t)
	schema := filepath.Join(root, "db", "schema.volt")
	d := NewDocument("file://"+schema, goRefSchema)
	if d.GoFilesChanged() {
		t.Fatal("fresh index reports stale Go files")
	}
	if d.Definition(posOf(t, goRefSchema, "EmailValid", 0)) == nil {
		t.Fatal("precondition: EmailValid resolves before the rename")
	}

	// gopls renames the function; the .volt buffer is untouched.
	goFile := filepath.Join(root, "db", "checks.go")
	src, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.ReplaceAll(string(src), "EmailValid", "AddressValid")
	if err := os.WriteFile(goFile, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(goFile, future, future); err != nil {
		t.Fatal(err)
	}

	if !d.GoFilesChanged() {
		t.Fatal("index did not notice the Go file changed")
	}
	d.Update(d.Text) // what the server does before answering
	if loc := d.Definition(posOf(t, goRefSchema, "EmailValid", 0)); loc != nil {
		t.Errorf("definition still resolves to the renamed function: %v", loc)
	}
	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, "no function EmailValid") {
			found = true
		}
	}
	if !found {
		t.Errorf("the reference must now be reported as undeclared; diags = %v", d.Diags)
	}
}
