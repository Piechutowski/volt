package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/ast"
)

const navSchema = "package db\n\ntable posts {\n\tid    int [pk]\n\ttitle string [null]\n}\n"
const navRoutes = "package app\n\nimport (\n\tdb\n)\n\npipeline api {\n\tuse volt.RequestID\n}\n\nscope / [pipe: api] {\n\tresources db.posts\n}\n"

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
	for _, want := range []string{"table posts", "package `db`", "Go model `Post`", "id int [pk]", "title string [null]"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	if !strings.Contains(md, "route parameter type `int`") {
		t.Errorf("hover should name the key's route parameter type:\n%s", md)
	}
}

func TestVoltHoverTableKeyNotRoutable(t *testing.T) {
	schema := "package db\n\ntable samples {\n\tid    float64 [pk]\n\tlabel string  [null]\n}\n"
	routes := "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tresources db.samples\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":          "module nav\n",
		"db/schema.volt":  schema,
		"app/routes.volt": routes,
	})
	d := NewDocument("file://"+filepath.Join(root, "app", "routes.volt"), routes)

	h := d.Hover(posOf(t, routes, "db.samples", 0))
	if h == nil {
		t.Fatal("no hover for db.samples")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "key: `id`\n") {
		t.Errorf("hover should name the key:\n%s", md)
	}
	if strings.Contains(md, "route parameter type") {
		t.Errorf("a float64 key is not a route parameter type:\n%s", md)
	}
}

// TestVoltHoverTableKeyFromPartials pins that the hover reads the key
// after partial injection (spec §columns): a key a partial injects is
// named, and a composite key split between the body and a partial is
// not named as the body's one pk column.
func TestVoltHoverTableKeyFromPartials(t *testing.T) {
	schema := "package db\n\ntable_partial daily {\n\tday int [pk]\n}\n\ntable site_days {\n\tsite string [pk]\n\t~daily\n\tviews int\n}\n\ntable_partial base {\n\tid int [pk]\n}\n\ntable posts {\n\t~base\n\ttitle string\n}\n"
	routes := "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tresources db.site_days\n\tresources db.posts\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":          "module nav\n",
		"db/schema.volt":  schema,
		"app/routes.volt": routes,
	})
	d := NewDocument("file://"+filepath.Join(root, "app", "routes.volt"), routes)

	h := d.Hover(posOf(t, routes, "db.site_days", 0))
	if h == nil {
		t.Fatal("no hover for db.site_days")
	}
	if md := h.Contents.(protocol.MarkupContent).Value; strings.Contains(md, "key:") {
		t.Errorf("a composite key (site, day) has no single key to name:\n%s", md)
	}
	h = d.Hover(posOf(t, routes, "db.posts", 0))
	if h == nil {
		t.Fatal("no hover for db.posts")
	}
	if md := h.Contents.(protocol.MarkupContent).Value; !strings.Contains(md, "key: `id` → route parameter type `int`") {
		t.Errorf("the key the partial injects is the table's key:\n%s", md)
	}
}

// TestTableKeyUnchecked pins the key read from source when the package
// has no checked model: partials injected in source order, a column of
// the body overriding a partial's, as §tablepartial resolves them.
func TestTableKeyUnchecked(t *testing.T) {
	schema := "package db\n\ntable_partial daily {\n\tday int [pk]\n\tsite string\n}\n\ntable site_days {\n\tsite string [pk]\n\t~daily\n\tviews int\n}\n\ntable_partial base {\n\tid int [pk]\n}\n\ntable posts {\n\t~base\n\ttitle string\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module nav\n",
		"db/schema.volt": schema,
	})
	pr, err := lang.Load(disk.Source{}, root)
	if err != nil {
		t.Fatal(err)
	}
	pkg := pr.PackageAt(filepath.Join(root, "db"))
	if pkg == nil || pkg.Schema() != nil {
		t.Fatalf("want the loaded, unchecked package db, got %v", pkg)
	}
	want := map[string]string{"site_days": "site, day", "posts": "id"}
	for _, d := range pkg.Merged().Decls {
		tbl, ok := d.(*ast.Table)
		if !ok {
			continue
		}
		var names []string
		for _, col := range tableKey(pkg, tbl) {
			names = append(names, col.Name.Name())
		}
		if got := strings.Join(names, ", "); got != want[tbl.Name.Name()] {
			t.Errorf("key of %s = (%s), want (%s)", tbl.Name.Name(), got, want[tbl.Name.Name()])
		}
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
	if !strings.Contains(md, "pipeline `api`") || !strings.Contains(md, "volt.RequestID") {
		t.Errorf("pipeline hover wrong:\n%s", md)
	}
}

func TestVoltHoverSchemaStillWorks(t *testing.T) {
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

	schemaText = strings.Replace(navSchema, "table posts", "table articles", 1)
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
	if items[0].Detail == nil || !strings.Contains(*items[0].Detail, "key id int") {
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

func TestVoltCompleteLeavesSchemaChains(t *testing.T) {
	root := voltProject(t, map[string]string{
		"go.mod": "module lsptest\n",
		"db/schema.volt": "package db\n\ntable users {\n\tid int [pk]\n\temail string [null]\n}\n\n" +
			"table posts {\n\tid int [pk]\n\tauthor_id int [null]\n}\n\nref: posts.author_id > users.id\n",
	})
	path := root + "/db/schema.volt"
	text := "package db\n\ntable users {\n\tid int [pk]\n\temail string [null]\n}\n\n" +
		"table posts {\n\tid int [pk]\n\tauthor_id int [null]\n}\n\nref: posts.author_id > users.\n"
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
		t.Errorf("schema ref-chain completion lost: %v", labels)
	}
}

const existsSchema = "package db\n\ntable posts {\n\tid    int [pk]\n\ttitle string\n}\n\n" +
	"table post_tags {\n\tpost_id int    [pk]\n\ttag     string [pk]\n}\n\n" +
	"ref: post_tags.post_id > posts.id [as: post, reverse: tags]\n\n" +
	"select tagged for posts where exists tags (tag = :tag?) [order: (id asc)]\n"

func existsDocument(t *testing.T, text string) *Document {
	t.Helper()
	root := voltProject(t, map[string]string{"go.mod": "module lsptest\n", "db/schema.volt": existsSchema})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
	if d.vpkg == nil {
		t.Fatal("project index should be active for this file")
	}
	return d
}

// An exists term's relation and the columns inside it resolve against
// the table the relation leads to (spec §predicates).
func TestVoltExistsResolvesTheRelatedTable(t *testing.T) {
	d := existsDocument(t, existsSchema)
	loc := d.Definition(posOf(t, existsSchema, "tag =", 0))
	if loc == nil {
		t.Fatal("no definition for the column inside exists")
	}
	if want := posOf(t, existsSchema, "tag     string", 0); loc.Range.Start.Line != want.Line {
		t.Errorf("the column inside exists resolves to line %d, want post_tags.tag on line %d", loc.Range.Start.Line, want.Line)
	}
	rel := d.Definition(posOf(t, existsSchema, "tags (", 0))
	if rel == nil || rel.Range.Start.Line != posOf(t, existsSchema, "post_tags {", 0).Line {
		t.Errorf("the relation of exists resolves to %v, want the table post_tags", rel)
	}
}

// Inside a predicate completion offers exists, and after exists the
// relations of the select's target (spec §predicates, §keywords).
func TestVoltCompleteExists(t *testing.T) {
	text := strings.Replace(existsSchema, "where exists tags (tag = :tag?)", "where ex", 1)
	d := existsDocument(t, text)
	pos := posOf(t, text, "where ex", 0)
	pos.Character += uint32(len("where ex"))
	labels := map[string]bool{}
	for _, it := range d.Complete(pos) {
		labels[it.Label] = true
	}
	if !labels["exists "] {
		t.Errorf("completion after where ex = %v, want exists", labels)
	}

	text = strings.Replace(existsSchema, "where exists tags (tag = :tag?)", "where exists t", 1)
	d = existsDocument(t, text)
	pos = posOf(t, text, "exists t", 0)
	pos.Character += uint32(len("exists t"))
	var got []string
	for _, it := range d.Complete(pos) {
		got = append(got, it.Label)
	}
	if len(got) != 1 || got[0] != "tags" {
		t.Errorf("completion after exists t = %v, want [tags]", got)
	}
}

// A pred named inside an exists term is a reference like any other:
// definition reaches the pred, references list the use, and a rename
// rewrites it (spec §predicates).
func TestVoltExistsPredReference(t *testing.T) {
	schema := "package db\n\ntable posts {\n\tid    int [pk]\n\ttitle string\n}\n\n" +
		"table post_tags {\n\tpost_id int    [pk]\n\ttag     string [pk]\n}\n\n" +
		"ref: post_tags.post_id > posts.id [as: post, reverse: tags]\n\n" +
		"pred golang { tag = \"go\" }\n\n" +
		"select tagged for posts where exists tags (golang) [order: (id asc)]\n" +
		"select direct for post_tags where golang\n"
	root := voltProject(t, map[string]string{"go.mod": "module lsptest\n", "db/schema.volt": schema})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), schema)
	decl := posOf(t, schema, "golang {", 0)
	inside := posOf(t, schema, "golang)", 0)
	outside := posOf(t, schema, "golang\n", 0)

	if loc := d.Definition(inside); loc == nil || loc.Range.Start.Line != decl.Line || loc.Range.Start.Character != decl.Character {
		t.Errorf("definition from the use inside exists = %v, want the pred on line %d", loc, decl.Line)
	}
	refs := d.References(decl, true)
	lines := map[protocol.UInteger]bool{}
	for _, r := range refs {
		lines[r.Range.Start.Line] = true
	}
	for _, at := range []protocol.Position{decl, inside, outside} {
		if !lines[at.Line] {
			t.Errorf("references %v miss line %d", refs, at.Line)
		}
	}
	edit, err := d.Rename(decl, "gopher")
	if err != nil {
		t.Fatal(err)
	}
	var renamed int
	for _, edits := range edit.Changes {
		for _, e := range edits {
			if e.Range.Start == inside {
				renamed++
			}
		}
	}
	if renamed != 1 {
		t.Errorf("rename leaves the use inside exists untouched: %v", edit.Changes)
	}
}

// A predicate term starts after a predicate's parenthesis too, where
// completion offers the preds and exists; inside an exists term, which
// does not nest, it offers the preds alone, and a projection's
// parenthesis is no predicate (spec §predicates).
func TestVoltCompletePredicateParens(t *testing.T) {
	schema := strings.Replace(existsSchema, "select tagged", "pred recent { id > 0 }\n\nselect tagged", 1)
	root := voltProject(t, map[string]string{"go.mod": "module lsptest\n", "db/schema.volt": schema})
	for _, tc := range []struct {
		line   string
		exists bool
		recent bool
	}{
		{"select feed for posts where (", true, true},
		{"select feed for posts where ((re", false, true},
		{"select feed for posts where title != \"\" or (ex", true, false},
		{"select feed for posts where not (", true, true},
		{"select feed for posts where exists tags (", false, true},
		{"select feed for posts where exists tags (tag = \"go\" and ex", false, false},
		{"select feed for posts where exists tags (tag = \"go\") and (ex", true, false},
		{"pred fresh { (", true, true},
		{"select feed (", false, false},
		{"select feed for posts where id in (", false, false},
		{"select feed for posts [order: (", false, false},
	} {
		text := strings.Replace(schema, "select tagged", tc.line+"\nselect tagged", 1)
		d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), text)
		pos := posOf(t, text, tc.line, 0)
		pos.Character += uint32(len(tc.line))
		labels := map[string]bool{}
		for _, it := range d.Complete(pos) {
			labels[it.Label] = true
		}
		if labels["exists "] != tc.exists || labels["recent"] != tc.recent {
			t.Errorf("completion after %q = %v, want exists %v and recent %v", tc.line, labels, tc.exists, tc.recent)
		}
	}
}
