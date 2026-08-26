package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const navSchema = "package db\n\nTable posts {\n\tid    integer [pk]\n\ttitle text\n}\n"
const navRoutes = "package app\n\nimport (\n\tdb\n)\n\nPipeline api {\n\tuse volt.RequestID\n}\n\nScope / [pipe: api] {\n\tresources db.Post\n}\n"

// navProject materializes the two-package project both tests use.
func navProject(t *testing.T) (root, schema, routes string) {
	t.Helper()
	root = voltProject(t, map[string]string{
		"volt.mod":        "module nav\n",
		"db/schema.volt":  navSchema,
		"app/routes.volt": navRoutes,
	})
	return root, filepath.Join(root, "db", "schema.volt"), filepath.Join(root, "app", "routes.volt")
}

// TestVoltDefinitionCrossPackage: go-to-definition on `db.Post` in
// routes.volt lands on `Table posts` in db/schema.volt — a different
// file, in a different package.
func TestVoltDefinitionCrossPackage(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	loc := d.Definition(posOf(t, navRoutes, "db.Post", 0))
	if loc == nil {
		t.Fatal("no definition for db.Post")
	}
	if loc.URI != "file://"+schema {
		t.Fatalf("definition URI = %s, want %s", loc.URI, "file://"+schema)
	}
	// "Table posts" is on line 3 (0-based line 2), name at column 6.
	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 6 {
		t.Errorf("definition range = %+v, want line 2 char 6", loc.Range.Start)
	}
}

// TestVoltDefinitionPipeline: `pipe: api` resolves to the Pipeline
// declaration in the same file.
func TestVoltDefinitionPipeline(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	loc := d.Definition(posOf(t, navRoutes, "api", 1)) // 0 is the declaration
	if loc == nil {
		t.Fatal("no definition for pipe: api")
	}
	if !strings.HasSuffix(loc.URI, "routes.volt") || loc.Range.Start.Line != 6 {
		t.Errorf("pipeline definition = %s %+v, want routes.volt line 6", loc.URI, loc.Range.Start)
	}
}

// TestVoltReferencesFindUses: asking for references on the table
// declaration in schema.volt finds the `resources db.Post` use over in
// app/routes.volt.
func TestVoltReferencesFindUses(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+schema, navSchema)

	refs := d.References(posOf(t, navSchema, "posts", 0), false)
	if len(refs) == 0 {
		t.Fatal("no references found for Table posts")
	}
	found := false
	for _, r := range refs {
		if r.URI == "file://"+routes {
			found = true
		}
	}
	if !found {
		t.Errorf("the resources use in routes.volt is missing; got %v", refs)
	}
}

// TestVoltDefinitionImport: the import path jumps to the package.
func TestVoltDefinitionImport(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	pos := posOf(t, navRoutes, "\tdb\n", 0)
	pos.Character++ // onto the identifier itself
	loc := d.Definition(pos)
	if loc == nil || loc.URI != "file://"+schema {
		t.Fatalf("import definition = %v, want %s", loc, "file://"+schema)
	}
}

// TestVoltNavOutsideProjectIsQuiet: with no volt.mod above it, the
// project index is absent and navigation must not panic or invent
// locations.
func TestVoltNavOutsideProjectIsQuiet(t *testing.T) {
	d := NewDocument("file:///lone.volt", navRoutes)
	if d.vindex != nil {
		t.Error("project index built for a file outside any project")
	}
	if loc := d.Definition(posOf(t, navRoutes, "db.Post", 0)); loc != nil {
		t.Errorf("definition = %v, want nil", loc)
	}
	if refs := d.References(posOf(t, navRoutes, "db.Post", 0), true); len(refs) != 0 {
		t.Errorf("references = %v, want none", refs)
	}
}

// TestVoltHoverTableExplainsModelName: hovering `db.Post` must show
// which table it means and how the name was derived — that mapping is
// invisible in routes.volt otherwise.
func TestVoltHoverTableExplainsModelName(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	h := d.Hover(posOf(t, navRoutes, "db.Post", 0))
	if h == nil {
		t.Fatal("no hover for db.Post")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"model `Post`", "Table posts", "package `db`", "singulariz"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	// The routed key type is what a handler signature will use.
	if !strings.Contains(md, "int32") {
		t.Errorf("hover should name the key's route parameter type:\n%s", md)
	}
}

// TestVoltHoverPipeline: hovering a pipe: reference lists its plugs.
func TestVoltHoverPipeline(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	h := d.Hover(posOf(t, navRoutes, "api", 1))
	if h == nil {
		t.Fatal("no hover for pipe: api")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "Pipeline `api`") || !strings.Contains(md, "volt.RequestID") {
		t.Errorf("pipeline hover wrong:\n%s", md)
	}
}

// TestVoltHoverDBMLStillWorks: the single-file DBML hover is unchanged
// for symbols the project index does not own.
func TestVoltHoverDBMLStillWorks(t *testing.T) {
	_, schema, _ := navProject(t)
	d := NewDocument("file://"+schema, navSchema)

	if h := d.Hover(posOf(t, navSchema, "title", 0)); h == nil {
		t.Error("column hover regressed")
	}
}
