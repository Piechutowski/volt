package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const groupSetSchema = "package db\n\n" +
	"Table ms_revenue {\n\tid int [pk]\n\tyear int\n}\n\n" +
	"Table ms_usage {\n\tid int [pk]\n\tyear int\n}\n\n" +
	"Table ms_dict {\n\tcode string [pk]\n}\n\n" +
	"Group Metrics {\n\tms_revenue\n\tms_usage\n\tms_dict\n}\n\n" +
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

func TestGroupTermDefinitionAndHover(t *testing.T) {
	d, text := groupSetDoc(t)
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	loc := d.Definition(posOf(t, text, "Metrics", 1))
	if loc == nil {
		t.Fatal("no definition for the group term")
	}
	if want := posOf(t, text, "Metrics", 0); loc.Range.Start != want {
		t.Errorf("definition at %+v, want the group declaration at %+v", loc.Range.Start, want)
	}
	h := d.Hover(posOf(t, text, "Metrics", 1))
	if h == nil {
		t.Fatal("no hover for the group term")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"Group Metrics", "- + ms_revenue", "- + ms_dict"} {
		if !strings.Contains(md, want) {
			t.Errorf("group hover missing %q:\n%s", want, md)
		}
	}

	loc = d.Definition(posOf(t, text, "Metrics", 2))
	if loc == nil || loc.Range.Start != posOf(t, text, "Metrics", 0) {
		t.Errorf("select target over a group did not resolve to its declaration: %+v", loc)
	}

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

func TestGroupRenameFollowsSetUses(t *testing.T) {
	d, text := groupSetDoc(t)
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

	edit, err = d.Rename(posOf(t, text, "ms_dict", 1), "ms_codes")
	if err != nil {
		t.Fatal(err)
	}
	n = 0
	for _, edits := range edit.Changes {
		n += len(edits)
	}
	if n < 3 {
		t.Errorf("table rename produced %d edits, want the declaration, the group member and the set term: %+v", n, edit.Changes)
	}
}

func TestDatasetNavigatesToSelect(t *testing.T) {
	const dbText = "package db\n\nTable ms_revenue {\n\tid int [pk]\n\tyear int\n}\n\n" +
		"Group series {\n\tms_revenue\n}\n\nSelect browse for series where year = :year\n"
	const appText = "package app\n\nimport (\n\tdb\n)\n\nScope /ms {\n\tdataset db.browse [strip: 'ms_']\n}\n"
	root := voltProject(t, map[string]string{
		"go.mod":          "module g\n",
		"db/schema.volt":  dbText,
		"app/routes.volt": appText,
	})
	d := NewDocument("file://"+filepath.Join(root, "app", "routes.volt"), appText)
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	loc := d.Definition(posOf(t, appText, "browse", 0))
	if loc == nil || !strings.HasSuffix(string(loc.URI), "db/schema.volt") {
		t.Fatalf("dataset select definition = %+v, want db/schema.volt", loc)
	}
	h := d.Hover(posOf(t, appText, "browse", 0))
	if h == nil || !strings.Contains(h.Contents.(protocol.MarkupContent).Value, "MsRevenueBrowse") {
		t.Fatalf("dataset hover should list the select's methods: %+v", h)
	}
	edit, err := d.Rename(posOf(t, appText, "browse", 0), "list")
	if err != nil {
		t.Fatal(err)
	}
	for uri, edits := range edit.Changes {
		for _, e := range edits {
			if strings.HasSuffix(string(uri), "routes.volt") && (e.Range.Start.Character != uint32(strings.Index("\tdataset db.browse", "browse")) || e.NewText != "list") {
				t.Errorf("dataset rename edit = %+v, want only the name after the qualifier", e)
			}
		}
	}
}
