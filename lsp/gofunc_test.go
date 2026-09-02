// Go-reference navigation (§V3.2, §V12.5): gd on a Go-reference check
// or a pipeline plug lands on the function in the package's Go file,
// and hover shows its signature and doc comment.
package lsp

import (
	"path/filepath"
	"strings"
	"testing"

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
