// TableGroups as sets (spec §V9.2, D65): a TableGroup name in a group
// expression or as a select target navigates, hovers, and renames like
// any other set, and the group hover spells the algebra as written.
package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const groupSetSchema = "package db\n\n" +
	"Table ms_revenue {\n\tid integer [pk]\n\tyear integer [not null]\n}\n\n" +
	"Table ms_usage {\n\tid integer [pk]\n\tyear integer [not null]\n}\n\n" +
	"Table ms_dict {\n\tcode text [pk]\n}\n\n" +
	"TableGroup Metrics {\n\tms_revenue\n\tms_usage\n\tms_dict\n}\n\n" +
	"Group series = Metrics \\ (ms_dict)\n\n" +
	"Select rows for series where year = :year\n" +
	"Select everything for Metrics\n"

func groupSetDoc(t *testing.T) (*Document, string) {
	t.Helper()
	root := voltProject(t, map[string]string{
		"go.mod":         "module g\n",
		"db/schema.volt": groupSetSchema,
	})
	return NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), groupSetSchema), groupSetSchema
}

func TestTableGroupTermDefinitionAndHover(t *testing.T) {
	d, text := groupSetDoc(t)
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// `Metrics` inside the group expression points at the TableGroup.
	loc := d.Definition(posOf(t, text, "Metrics", 1))
	if loc == nil {
		t.Fatal("no definition for the TableGroup term")
	}
	if want := posOf(t, text, "Metrics", 0); loc.Range.Start != want {
		t.Errorf("definition at %+v, want the TableGroup declaration at %+v", loc.Range.Start, want)
	}
	h := d.Hover(posOf(t, text, "Metrics", 1))
	if h == nil {
		t.Fatal("no hover for the TableGroup term")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"TableGroup Metrics", "- ms_revenue", "- ms_dict"} {
		if !strings.Contains(md, want) {
			t.Errorf("TableGroup hover missing %q:\n%s", want, md)
		}
	}

	// The select target `Metrics` resolves the same way.
	loc = d.Definition(posOf(t, text, "Metrics", 2))
	if loc == nil || loc.Range.Start != posOf(t, text, "Metrics", 0) {
		t.Errorf("select target over a TableGroup did not resolve to its declaration: %+v", loc)
	}

	// The group hover spells the algebra as written: '\' and the set.
	h = d.Hover(posOf(t, text, "series", 0))
	if h == nil {
		t.Fatal("no hover for the group")
	}
	md = h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"- + Metrics", "- \\ (ms_dict)"} {
		if !strings.Contains(md, want) {
			t.Errorf("group hover missing %q:\n%s", want, md)
		}
	}
}

func TestTableGroupRenameFollowsSetUses(t *testing.T) {
	d, text := groupSetDoc(t)
	// Declaration + group term + select target = 3 edits.
	edit, err := d.Rename(posOf(t, text, "Metrics", 1), "Series")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n != 3 {
		t.Errorf("rename produced %d edits, want 3 (declaration, group term, select target): %+v", n, edit.Changes)
	}

	// A table inside a set term renames with its declaration.
	edit, err = d.Rename(posOf(t, text, "ms_dict", 1), "ms_codes")
	if err != nil {
		t.Fatal(err)
	}
	n = 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n < 3 {
		t.Errorf("table rename produced %d edits, want the declaration, the TableGroup member and the set term: %+v", n, edit.Changes)
	}
}
