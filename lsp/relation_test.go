package lsp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lsp/protocol"
)

const relationSchema = "package db\n\n" +
	"table users {\n\tid   int [pk]\n\tname string\n}\n\n" +
	"table posts {\n\tid        int    [pk]\n\tauthor_id int    [null, ref: users.id]\n\twriter_id int    [null]\n\teditor_id int    [null]\n\ttitle     string\n}\n\n" +
	"table notes {\n\tid      int    [pk]\n\tpost_id int\n\tbody    string\n}\n\n" +
	"table comments {\n\tid      int    [pk]\n\tpost_id int    [ref: posts.id]\n\tbody    string\n}\n\n" +
	"ref: posts.writer_id > users.id [as: writer, reverse: written]\n" +
	"ref: posts.editor_id > users.id [as: editor, reverse: edited]\n" +
	"ref: notes.post_id > posts.id [as: post, reverse: remarks]\n\n" +
	"pred by_writer { writer.name = :n }\n" +
	"pred has_body { exists comments (body != \"\") }\n\n" +
	"select written_by (id, writer.name) for posts join writer where by_writer [order: (writer.name asc, id asc)]\n" +
	"select remarked for posts where exists remarks (body != \"\") [order: (id asc)]\n" +
	"select commented for posts where has_body [order: (id asc)]\n"

const relationSelects = "package db\n\nselect by_author (id, title, author.name) for posts join author [order: (id asc)]\n"

func relationProject(t *testing.T, schema string) (string, *Document) {
	t.Helper()
	root := voltProject(t, map[string]string{"go.mod": "module rel\n", "db/schema.volt": schema, "db/selects.volt": relationSelects})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), schema)
	if d.vpkg == nil {
		t.Fatal("project index should be active for this file")
	}
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("the project does not check before any rename: %v", diags)
	}
	return root, d
}

func renameApplied(t *testing.T, root string, d *Document, edit *protocol.WorkspaceEdit) []diag.Diagnostic {
	t.Helper()
	overlay := map[string]string{}
	for uri, edits := range edit.Changes {
		path := pathFromURI(uri)
		text := d.Text
		if path != pathFromURI(d.URI) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text = string(b)
		}
		doc := NewDocument("file:///none.volt", text)
		var es []renameEdit
		for _, e := range edits {
			es = append(es, renameEdit{path, doc.FromLSP(e.Range.Start), doc.FromLSP(e.Range.End), e.NewText})
		}
		overlay[path] = editsApply(text, es)
	}
	pr, err := lang.LoadOverlay(disk.Source{}, root, overlay)
	if err != nil {
		t.Fatal(err)
	}
	return errorsOf(lang.Check(pr))
}

func editCount(edit *protocol.WorkspaceEdit) int {
	n := 0
	for _, es := range edit.Changes {
		n += len(es)
	}
	return n
}

// TestRelationRenameAtItsDeclaration proves a rename at the as: or
// reverse: that names a relation (spec §relationships-ref, editor.md
// section 7): it rewrites the declaration and every use, a pred body's
// among them, and the project still checks.
func TestRelationRenameAtItsDeclaration(t *testing.T) {
	for _, tc := range []struct {
		at, to string
		edits  int
	}{
		{"writer, reverse", "author_of", 5},
		{"remarks]", "replies", 2},
		{"writer.name =", "penned", 5},
	} {
		root, d := relationProject(t, relationSchema)
		edit, err := d.Rename(posOf(t, relationSchema, tc.at, 0), tc.to)
		if err != nil {
			t.Fatalf("rename at %q: %v", tc.at, err)
		}
		if n := editCount(edit); n != tc.edits {
			t.Errorf("rename at %q edits %d places, want %d: %+v", tc.at, n, tc.edits, edit.Changes)
		}
		if errs := renameApplied(t, root, d, edit); len(errs) != 0 {
			t.Errorf("after the rename at %q the project fails: %v", tc.at, errs)
		}
	}
}

// TestRenameRefusesAnAmbiguousName proves that a rename never lands a
// name the project would refuse (spec §relationships-ref): a
// relation renamed to another of its table's relations or to the table's
// own name, and a table renamed to a relation name its reverse relation
// would then share, are refused, naming the error.
func TestRenameRefusesAnAmbiguousName(t *testing.T) {
	for _, tc := range []struct{ at, to, want string }{
		{"writer where", "editor", "ambiguous"},
		{"writer where", "posts", "own name"},
		{"writer where", "remarks", "ambiguous"},
		{"comments {", "remarks", "ambiguous"},
	} {
		_, d := relationProject(t, relationSchema)
		if _, err := d.Rename(posOf(t, relationSchema, tc.at, 0), tc.to); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("rename at %q to %s = %v; want a refusal saying %q", tc.at, tc.to, err, tc.want)
		}
	}
}

// TestRenameFollowsPredBodies proves that a relation, or a table that
// names one, renames in the pred bodies the selects over it read (spec
// §predicates): the qualifier of a column and the relation of an exists.
func TestRenameFollowsPredBodies(t *testing.T) {
	for _, tc := range []struct{ at, to, inPred string }{
		{"writer where", "author_of", "pred by_writer { author_of.name"},
		{"comments {", "replies", "pred has_body { exists replies ("},
		{"comments (body", "replies", "pred has_body { exists replies ("},
	} {
		root, d := relationProject(t, relationSchema)
		edit, err := d.Rename(posOf(t, relationSchema, tc.at, 0), tc.to)
		if err != nil {
			t.Fatalf("rename at %q: %v", tc.at, err)
		}
		if errs := renameApplied(t, root, d, edit); len(errs) != 0 {
			t.Errorf("after the rename at %q the project fails: %v", tc.at, errs)
		}
		var es []renameEdit
		for _, e := range edit.Changes[d.URI] {
			es = append(es, renameEdit{"", d.FromLSP(e.Range.Start), d.FromLSP(e.Range.End), e.NewText})
		}
		if got := editsApply(d.Text, es); !strings.Contains(got, tc.inPred) {
			t.Errorf("rename at %q leaves the pred unrenamed:\n%s", tc.at, got)
		}
	}
}

// TestColumnRelationAcrossFiles proves the two sides of a relation a
// column names (spec §relationships-ref): renaming it is
// refused from a file that does not declare the column too, and renaming
// the column renames its uses in every file, so the project checks; a
// column renamed at its use in another file renames its declaration.
func TestColumnRelationAcrossFiles(t *testing.T) {
	root, d := relationProject(t, relationSchema)
	sel := NewDocument("file://"+filepath.Join(root, "db", "selects.volt"), relationSelects)
	for _, at := range []string{"author [order", "author.name"} {
		if _, err := sel.Rename(posOf(t, relationSelects, at, 0), "writer2"); err == nil || !strings.Contains(err.Error(), "author_id") {
			t.Errorf("rename of the relation at %q in another file = %v; want a refusal naming the column author_id", at, err)
		}
	}
	edit, err := d.Rename(posOf(t, relationSchema, "author_id", 0), "byline_id")
	if err != nil {
		t.Fatal(err)
	}
	selEdits := edit.Changes[protocol.DocumentUri("file://"+filepath.Join(root, "db", "selects.volt"))]
	if len(selEdits) != 2 || slices.ContainsFunc(selEdits, func(e protocol.TextEdit) bool { return e.NewText != "byline" }) {
		t.Errorf("renaming author_id rewrites the relation in selects.volt as %+v; want its two uses renamed byline", selEdits)
	}
	if errs := renameApplied(t, root, d, edit); len(errs) != 0 {
		t.Errorf("after renaming the column the project fails: %v", errs)
	}
	edit, err = sel.Rename(posOf(t, relationSelects, "title", 0), "heading")
	if err != nil {
		t.Fatalf("renaming a column at its use in another file: %v", err)
	}
	if len(edit.Changes) != 2 {
		t.Errorf("renaming title at its use edits %v; want its declaration and its use, in two files", edit.Changes)
	}
	if errs := renameApplied(t, root, sel, edit); len(errs) != 0 {
		t.Errorf("after renaming title the project fails: %v", errs)
	}
}

// TestRelationHoverAndDefinition proves a relation's navigation (spec
// §relationships-ref): it hovers as itself and the table it leads to,
// goes to the as: or reverse: that names it or to the column its name
// derives from, and lists its declaration once among its references.
func TestRelationHoverAndDefinition(t *testing.T) {
	root, _ := relationProject(t, relationSchema)
	sel := NewDocument("file://"+filepath.Join(root, "db", "selects.volt"), relationSelects)
	h := sel.Hover(posOf(t, relationSelects, "author [order", 0))
	if h == nil || !strings.Contains(h.Contents.Value, "relation `author`") || !strings.Contains(h.Contents.Value, "table users") {
		t.Errorf("hover at a relation a column names = %+v; want the relation and the table users", h)
	}
	loc := sel.Definition(posOf(t, relationSelects, "author.name", 0))
	if want := posOf(t, relationSchema, "author_id", 0); loc == nil || !strings.HasSuffix(string(loc.URI), "schema.volt") || loc.Range.Start != want {
		t.Errorf("definition of the relation author = %+v; want the column author_id at %v", loc, want)
	}
	_, d := relationProject(t, relationSchema)
	if h := d.Hover(posOf(t, relationSchema, "remarks (body", 0)); h == nil || !strings.Contains(h.Contents.Value, "table notes") {
		t.Errorf("hover at a relation reverse: names = %+v; want the table notes it leads to", h)
	}
	refs := d.References(posOf(t, relationSchema, "writer where", 0), true)
	seen := map[protocol.Location]bool{}
	for _, r := range refs {
		if seen[r] {
			t.Errorf("references list %v twice: %v", r, refs)
		}
		seen[r] = true
	}
	if len(refs) != 5 {
		t.Errorf("references of writer = %v; want its as: and four uses", refs)
	}
}

// TestRenameRefusesAnErrorBesideOneItFixes proves that a rename lands
// only when it adds no error (editor.md section 7): beside an error it
// fixes, a relation renamed so that a pred another select reads no
// longer finds it is refused, naming the error it adds, and a rename
// that adds none lands beside an error it leaves as it was.
func TestRenameRefusesAnErrorBesideOneItFixes(t *testing.T) {
	schema := "package db\n\n" +
		"table users {\n\tid   int [pk]\n\tname string\n}\n\n" +
		"table posts {\n\tid        int [pk]\n\teditor_id int\n}\n\n" +
		"table pages {\n\tid        int [pk]\n\teditor_id int\n}\n\n" +
		"ref: posts.editor_id > users.id [as: editor, reverse: posts_edited]\n" +
		"ref: pages.editor_id > users.id [as: editor, reverse: pages_edited]\n\n" +
		"select p (id, editor.name) for posts join editor where by_editor [order: (id asc)]\n" +
		"select g (id, editor.name) for pages join editor where by_editor [order: (id asc)]\n" +
		"select typo (id, reviser.name) for posts join reviser [order: (id asc)]\n"
	root := voltProject(t, map[string]string{"go.mod": "module rel\n", "db/schema.volt": schema, "db/preds.volt": "package db\n\npred by_editor { editor.name = :n }\n"})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), schema)
	if n := len(d.LSPDiagnostics()); n != 1 {
		t.Fatalf("the project checks with %d errors before the rename; want the one of select typo", n)
	}
	_, err := d.Rename(posOf(t, schema, "editor, reverse: posts_edited", 0), "reviser")
	if err == nil || !strings.Contains(err.Error(), "by_editor") {
		t.Errorf("renaming posts' editor to reviser = %v; want a refusal naming the pred by_editor, which select g still reads as editor", err)
	}
	if _, err := d.Rename(posOf(t, schema, "pages_edited", 0), "pages_kept"); err != nil {
		t.Errorf("renaming a relation no select reads beside the error of select typo = %v; want it to land", err)
	}
}

// TestRelationNamedAfterATableHovers proves that a relation named after
// the table it leads to hovers as the relationship it walks and then
// the table (editor.md section 7), as one named by as: does, while it
// stays that table's symbol for rename and definition.
func TestRelationNamedAfterATableHovers(t *testing.T) {
	_, d := relationProject(t, relationSchema)
	h := d.Hover(posOf(t, relationSchema, "comments (body", 0))
	if h == nil {
		t.Fatal("no hover at a relation named after its table")
	}
	md := h.Contents.Value
	if !strings.Contains(md, "relation `comments`") || !strings.Contains(md, "comments.post_id") || !strings.Contains(md, "table comments") {
		t.Errorf("hover at exists comments = %q; want the relation, the foreign key it walks and the table", md)
	}
	if h := d.Hover(posOf(t, relationSchema, "comments {", 0)); h == nil || strings.Contains(h.Contents.Value, "relation `") {
		t.Errorf("hover at the table's declaration = %+v; want the table alone", h)
	}
}
