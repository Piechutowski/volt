package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const navSchema = "package db\n\nTable posts {\n\tid    integer [pk]\n\ttitle text\n}\n"
const navRoutes = "package app\n\nimport (\n\tdb\n)\n\nPipeline api {\n\tuse volt.RequestID\n}\n\nScope / [pipe: api] {\n\tresources db.posts\n}\n"

func navProject(t *testing.T) (root, schema, routes string) {
	t.Helper()
	root = voltProject(t, map[string]string{
		"go.mod":          "module nav\n",
		"db/schema.volt":  navSchema,
		"app/routes.volt": navRoutes,
	})
	return root, filepath.Join(root, "db", "schema.volt"), filepath.Join(root, "app", "routes.volt")
}

func TestVoltDefinitionCrossPackage(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	loc := d.Definition(posOf(t, navRoutes, "db.posts", 0))
	if loc == nil {
		t.Fatal("no definition for db.posts")
	}
	if loc.URI != "file://"+schema {
		t.Fatalf("definition URI = %s, want %s", loc.URI, "file://"+schema)
	}
	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 6 {
		t.Errorf("definition range = %+v, want line 2 char 6", loc.Range.Start)
	}
}

func TestVoltDefinitionPipeline(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	loc := d.Definition(posOf(t, navRoutes, "api", 1))
	if loc == nil {
		t.Fatal("no definition for pipe: api")
	}
	if !strings.HasSuffix(loc.URI, "routes.volt") || loc.Range.Start.Line != 6 {
		t.Errorf("pipeline definition = %s %+v, want routes.volt line 6", loc.URI, loc.Range.Start)
	}
}

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

func TestVoltDefinitionImport(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	pos := posOf(t, navRoutes, "\tdb\n", 0)
	pos.Character++
	loc := d.Definition(pos)
	if loc == nil || loc.URI != "file://"+schema {
		t.Fatalf("import definition = %v, want %s", loc, "file://"+schema)
	}
}

func TestVoltNavOutsideProjectIsQuiet(t *testing.T) {
	d := NewDocument("file:///lone.volt", navRoutes)
	if d.vindex != nil {
		t.Error("project index built for a file outside any project")
	}
	if loc := d.Definition(posOf(t, navRoutes, "db.posts", 0)); loc != nil {
		t.Errorf("definition = %v, want nil", loc)
	}
	if refs := d.References(posOf(t, navRoutes, "db.posts", 0), true); len(refs) != 0 {
		t.Errorf("references = %v, want none", refs)
	}
}

func TestVoltHoverTableShowsModel(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	h := d.Hover(posOf(t, navRoutes, "db.posts", 0))
	if h == nil {
		t.Fatal("no hover for db.posts")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"Table posts", "package `db`", "Go model `Post`"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	if !strings.Contains(md, "int32") {
		t.Errorf("hover should name the key's route parameter type:\n%s", md)
	}
}

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

func TestVoltHoverDBMLStillWorks(t *testing.T) {
	_, schema, _ := navProject(t)
	d := NewDocument("file://"+schema, navSchema)

	if h := d.Hover(posOf(t, navSchema, "title", 0)); h == nil {
		t.Error("column hover regressed")
	}
}

func TestVoltRenameCrossFile(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+schema, navSchema)

	edit, err := d.Rename(posOf(t, navSchema, "posts", 0), "articles")
	if err != nil {
		t.Fatalf("rename from the declaration failed: %v", err)
	}
	if len(edit.Changes) != 2 {
		t.Fatalf("edits touch %d file(s), want 2: %v", len(edit.Changes), edit.Changes)
	}
	for _, uri := range []string{"file://" + schema, "file://" + routes} {
		edits := edit.Changes[protocol.DocumentUri(uri)]
		if len(edits) == 0 {
			t.Errorf("no edits for %s", uri)
			continue
		}
		for _, e := range edits {
			if e.NewText != "articles" {
				t.Errorf("%s: NewText = %q", uri, e.NewText)
			}
		}
	}
}

func TestVoltRenameFromReference(t *testing.T) {
	_, schema, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	edit, err := d.Rename(posOf(t, navRoutes, "db.posts", 0), "articles")
	if err != nil {
		t.Fatalf("rename from the reference failed: %v", err)
	}
	if len(edit.Changes) != 2 {
		t.Fatalf("edits touch %d file(s), want 2: %v", len(edit.Changes), edit.Changes)
	}
	if len(edit.Changes[protocol.DocumentUri("file://"+schema)]) != 1 {
		t.Errorf("the declaration in schema.volt was not renamed")
	}
}

func TestVoltRenameKeepsQualifier(t *testing.T) {
	_, _, routes := navProject(t)
	d := NewDocument("file://"+routes, navRoutes)

	edit, err := d.Rename(posOf(t, navRoutes, "db.posts", 0), "articles")
	if err != nil {
		t.Fatal(err)
	}
	line := 0
	for i, l := range strings.Split(navRoutes, "\n") {
		if strings.Contains(l, "db.posts") {
			line = i
		}
	}
	nameCol := strings.Index(strings.Split(navRoutes, "\n")[line], "posts")
	for _, e := range edit.Changes[protocol.DocumentUri("file://"+routes)] {
		if int(e.Range.Start.Line) != line {
			continue
		}
		if int(e.Range.Start.Character) != nameCol {
			t.Errorf("edit starts at char %d, want %d — the qualifier would be rewritten", e.Range.Start.Character, nameCol)
		}
		if got := int(e.Range.End.Character - e.Range.Start.Character); got != len("posts") {
			t.Errorf("edit spans %d chars, want %d", got, len("posts"))
		}
	}
}

func TestSiblingDiagnosticsFollowSchemaEdits(t *testing.T) {
	_, schema, routes := navProject(t)

	schemaText := navSchema
	d := &Document{URI: "file://" + routes, Siblings: func() map[string]string {
		return map[string]string{schema: schemaText}
	}}
	d.Update(navRoutes)
	if len(d.Diags) != 0 {
		t.Fatalf("clean project reports %v", d.Diags)
	}

	schemaText = strings.Replace(navSchema, "Table posts", "Table articles", 1)
	d.Update(d.Text)
	found := false
	for _, dg := range d.Diags {
		if strings.Contains(dg.Msg, `no table "posts"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("stale: schema renamed but no error; diags = %v", d.Diags)
	}

	schemaText = navSchema
	d.Update(d.Text)
	if len(d.Diags) != 0 {
		t.Fatalf("stale after revert: %v", d.Diags)
	}
}

func TestVoltCompleteQualifiedTables(t *testing.T) {
	_, _, routes := navProject(t)
	text := strings.Replace(navRoutes, "resources db.posts", "resources db.P", 1)
	d := NewDocument("file://"+routes, text)

	pos := posOf(t, text, "db.P", 0)
	pos.Character += 4
	items := d.Complete(pos)
	if len(items) != 1 || items[0].Label != "posts" {
		t.Fatalf("completion after db.P = %v, want [posts]", items)
	}
	if items[0].Detail == nil || !strings.Contains(*items[0].Detail, "key id int32") {
		t.Errorf("detail = %v, want the key type", items[0].Detail)
	}
}

func TestVoltCompleteResourcesOffersTablesAndImports(t *testing.T) {
	_, _, routes := navProject(t)
	text := strings.Replace(navRoutes, "resources db.posts", "resources ", 1)
	d := NewDocument("file://"+routes, text)

	pos := posOf(t, text, "resources ", 0)
	pos.Character += uint32(len("resources "))
	labels := map[string]bool{}
	for _, it := range d.Complete(pos) {
		labels[it.Label] = true
	}
	if !labels["db."] {
		t.Errorf("import qualifier missing from %v", labels)
	}
}

func TestVoltCompleteLeavesDBMLChains(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod": "module lsptest\n",
		"db/schema.volt": "package db\n\nTable users {\n\tid integer [pk]\n\temail text\n}\n\n" +
			"Table posts {\n\tid integer [pk]\n\tauthor_id integer\n}\n\nRef: posts.author_id > users.id\n",
	})
	path := root + "/db/schema.volt"
	text := "package db\n\nTable users {\n\tid integer [pk]\n\temail text\n}\n\n" +
		"Table posts {\n\tid integer [pk]\n\tauthor_id integer\n}\n\nRef: posts.author_id > users.\n"
	d := NewDocument("file://"+path, text)
	if d.vpkg == nil {
		t.Fatal("project index should be active for this file")
	}

	pos := posOf(t, text, "> users.", 0)
	pos.Character += uint32(len("> users."))
	labels := map[string]bool{}
	for _, it := range d.Complete(pos) {
		labels[it.Label] = true
	}
	if !labels["id"] || !labels["email"] {
		t.Errorf("DBML ref-chain completion lost: %v", labels)
	}
}
