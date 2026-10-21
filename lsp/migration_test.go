package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const migrationSchema = "package db\n\nTable users {\n\tid int [pk, increment]\n\tbio string [null]\n}\n\n" +
	"Migration 0002__users_bio [irreversible] {\n\talter table users {\n\t\tadd bio string [null]\n\t}\n\trebuild table users {\n\t\tid int [pk, increment]\n\t\tbio string [null]\n\t}\n}\n\n" +
	"Migration 0001__create_users {\n\tcreate table users {\n\t\tid int [pk, increment]\n\t}\n}\n"

func TestDocumentSymbolsMigration(t *testing.T) {
	d := doc(t, migrationSchema)
	got := map[string]protocol.DocumentSymbol{}
	for _, s := range d.DocumentSymbols() {
		got[s.Name] = s
	}
	for _, want := range []string{"0001__create_users", "0002__users_bio"} {
		s, ok := got[want]
		if !ok {
			t.Errorf("outline missing Migration %q: %v", want, got)
			continue
		}
		if s.Detail == nil || *s.Detail != "Migration" || s.Kind != protocol.SymbolKindModule {
			t.Errorf("Migration %q detail = %v kind = %v, want Migration, Module", want, s.Detail, s.Kind)
		}
	}
}

func TestCompletionTopLevelMigration(t *testing.T) {
	text := sample + "\nM"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: protocol.UInteger(strings.Count(text, "\n")), Character: 1})
	if !hasLabel(items, "Migration") {
		t.Fatalf("top-level completion missing Migration: %v", labels(items))
	}
}

func hasWord(items []protocol.CompletionItem, word string) bool {
	for _, it := range items {
		if strings.TrimSpace(it.Label) == word {
			return true
		}
	}
	return false
}

func TestCompletionMigrationBody(t *testing.T) {
	text := "Migration 0001__a {\n\t\n}\n"
	d := doc(t, text)
	items := d.Complete(posOf(t, text, "\t", 0))
	for _, want := range []string{"create table", "alter table", "exec", "up {"} {
		if !hasWord(items, want) {
			t.Errorf("migration body completion missing %q; got %v", want, labels(items))
		}
	}
	text = "Migration 0001__a {\n\talter table t {\n\t\t\n\t}\n}\n"
	d = doc(t, text)
	items = d.Complete(posOf(t, text, "\t\t", 0))
	if !hasWord(items, "add") || !hasWord(items, "rename") {
		t.Errorf("alter body completion = %v, want add, drop, rename", labels(items))
	}
	text = "Migration 0001__a [\n"
	d = doc(t, text)
	pos := posOf(t, text, "[", 0)
	pos.Character++
	items = d.Complete(pos)
	if !hasWord(items, "irreversible") || !hasWord(items, "no transaction") {
		t.Errorf("migration settings completion = %v", labels(items))
	}
}

func TestCompletionMigrationShapeSettings(t *testing.T) {
	text := "Migration 0001__a {\n\tcreate table t {\n\t\tid int [\n\t\tindexes {\n\t\t\tid [\n\t\t}\n\t}\n" +
		"\talter table t {\n\t\tadd bio string [\n\t}\n\trebuild table t {\n\t\tid int [\n\t}\n\tcreate index t id [\n}\n"
	d := doc(t, text)
	cases := []struct {
		where         string
		bracket       int
		want, refused []string
	}{
		{"create table column", 0, []string{"ref:", "delete:", "update:"}, []string{"required", "tag:", "from:", "note:"}},
		{"shape indexes", 1, []string{"pk", "where:", "name:"}, []string{"type:"}},
		{"alter add", 2, []string{"default:", "ref:"}, []string{"required", "tag:", "from:"}},
		{"rebuild column", 3, []string{"from:", "ref:"}, []string{"required", "tag:"}},
		{"standalone create index", 4, []string{"unique", "name:", "where:"}, []string{"pk", "type:"}},
	}
	for _, c := range cases {
		pos := posOf(t, text, "[", c.bracket)
		pos.Character++
		items := d.Complete(pos)
		for _, w := range c.want {
			if !hasWord(items, w) {
				t.Errorf("%s: completion lacks %q; got %v", c.where, w, labels(items))
			}
		}
		for _, r := range c.refused {
			if hasWord(items, r) {
				t.Errorf("%s: completion offers %q, which is not a migration setting; got %v", c.where, r, labels(items))
			}
		}
	}
	text = "Migration 0001__a {\n\tcreate table t {\n\t\tid i\n\t\t\n\t}\n}\n"
	d = doc(t, text)
	items := d.Complete(protocol.Position{Line: 2, Character: protocol.UInteger(len("\t\tid i"))})
	if !hasWord(items, "int") || len(items) != len(builtinTypes) {
		t.Errorf("shape column type completion = %v, want the built-ins only", labels(items))
	}
	items = d.Complete(protocol.Position{Line: 3, Character: 2})
	if !hasWord(items, "indexes") || len(items) != 1 {
		t.Errorf("shape body completion = %v, want indexes only", labels(items))
	}
}

func TestCompletionIndexWhere(t *testing.T) {
	text := "Table t {\n\tid int\n\tindexes {\n\t\tid [\n\t}\n}\n"
	d := doc(t, text)
	pos := posOf(t, text, "[", 0)
	pos.Character++
	if items := d.Complete(pos); !hasWord(items, "where:") {
		t.Fatalf("index settings completion missing where:; got %v", labels(items))
	}
}

func TestMigrationHoverSingleFile(t *testing.T) {
	d := NewDocument("file:///lone.volt", migrationSchema)
	h := d.Hover(posOf(t, migrationSchema, "0002__users_bio", 0))
	if h == nil {
		t.Fatal("no hover on the migration name")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"Migration 0002__users_bio", "migration 2 of 2", "irreversible"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover lacks %q:\n%s", want, md)
		}
	}
	h = d.Hover(posOf(t, migrationSchema, "0001__create_users", 0))
	if h == nil {
		t.Fatal("no hover on the first migration")
	}
	md = h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "migration 1 of 2, reversible") {
		t.Errorf("hover on the first migration:\n%s", md)
	}
}

func TestMigrationHoverCountsThePackage(t *testing.T) {
	const later = "package db\n\nMigration 0003__drop_bio [irreversible] {\n\talter table users {\n\t\tdrop bio\n\t}\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module m\n",
		"db/schema.volt": migrationSchema,
		"db/later.volt":  later,
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "later.volt"), later)
	h := d.Hover(posOf(t, later, "0003__drop_bio", 0))
	if h == nil {
		t.Fatal("no hover on a migration in its own file")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "migration 3 of 3, irreversible") {
		t.Errorf("hover does not count the package's migrations:\n%s", md)
	}
	if len(d.Diags) != 0 {
		t.Errorf("a migration alone in its file is checked with its package, got %v", d.Diags)
	}
}
