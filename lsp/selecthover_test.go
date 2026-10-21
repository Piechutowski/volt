package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const projSchema = "package db\n\n" +
	"Table page_views {\n\tid int [pk, increment]\n\tsite string [not null]\n\tday int [not null]\n\thits int [not null, default: 0]\n}\n\n" +
	"Table link_clicks {\n\tid int [pk, increment]\n\tsite string [not null]\n\tday int [not null]\n\ttarget string [not null, default: '']\n}\n\n" +
	"Group metrics {\n\tpage_views\n\tlink_clicks\n}\n\n" +
	"Select summary (site, day) for metrics where day >= :from\n" +
	"Select public (* \\ site) for link_clicks\n" +
	"Select chosen for metrics where site in :sites and day in :days\n" +
	"Select busy for page_views where hits > 10 and site = :site and day in :days\n"

func projSelectDoc(t *testing.T) (*Document, string) {
	t.Helper()
	root := voltProject(t, map[string]string{
		"go.mod":         "module proj\n",
		"db/schema.volt": projSchema,
	})
	path := filepath.Join(root, "db", "schema.volt")
	return NewDocument("file://"+path, projSchema), projSchema
}

func TestSelectHoverSharedRowType(t *testing.T) {
	d, text := projSelectDoc(t)
	h := d.Hover(posOf(t, text, "summary", 0))
	if h == nil {
		t.Fatal("no hover for the summary select")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{
		"PageViewSummary(ctx context.Context, from int) ([]Summary, error)",
		"LinkClickSummary(ctx context.Context, from int) ([]Summary, error)",
		"type Summary struct",
		"Site string",
		"Day int",
		"WHERE \"day\" >= ?\n```\nbinds: `from`\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	if strings.Count(md, "type Summary struct") != 1 {
		t.Errorf("shared row type rendered more than once:\n%s", md)
	}
	structAt := strings.Index(md, "type Summary struct")
	whereAt := strings.Index(md, "WHERE ")
	funcAt := strings.Index(md, "func (q *Queries)")
	if !(structAt < whereAt && whereAt < funcAt) {
		t.Errorf("hover order must be struct, SQL, signatures; got offsets %d %d %d:\n%s", structAt, whereAt, funcAt, md)
	}
}

func TestSelectHoverStructDerivative(t *testing.T) {
	d, text := projSelectDoc(t)
	h := d.Hover(posOf(t, text, "public", 0))
	if h == nil {
		t.Fatal("no hover for the public select")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{
		"LinkClickPublic(ctx context.Context) ([]LinkClickPublic, error)",
		"type LinkClickPublic struct",
		"Target string",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Site") {
		t.Errorf("excluded column leaked into the derivative hover:\n%s", md)
	}
}

func TestSelectHoverListParam(t *testing.T) {
	d, text := projSelectDoc(t)
	h := d.Hover(posOf(t, text, "chosen", 0))
	if h == nil {
		t.Fatal("no hover for the chosen select")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{
		"PageViewChosen(ctx context.Context, sites []string, days []int) ([]PageView, error)",
		"json_each(?)",
		"binds: `rt.JSONList(sites)`, `rt.JSONList(days)`",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
}

func TestSelectHoverBindsInPlaceholderOrder(t *testing.T) {
	d, text := projSelectDoc(t)
	h := d.Hover(posOf(t, text, "busy", 0))
	if h == nil {
		t.Fatal("no hover for the busy select")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	want := "```sql\nWHERE (\"hits\" > ? AND \"site\" = ? AND \"day\" IN (SELECT value FROM json_each(?)))\n```\n" +
		"binds: `10`, `site`, `rt.JSONList(days)`\n"
	if !strings.Contains(md, want) {
		t.Errorf("hover missing %q:\n%s", want, md)
	}
	if !strings.Contains(md, "PageViewBusy(ctx context.Context, site string, days []int) ([]PageView, error)") {
		t.Errorf("hover missing the signature:\n%s", md)
	}
}

const unresolvedSchema = "package db\n\n" +
	"Table authors {\n\tid int [pk, increment]\n\temail string [not null]\n\trank int32 [not null]\n}\n\n" +
	"Select ranked for authors where rank >= :min and email = :e [order: (rank desc)]\n" +
	"Select shown (rank, email) for authors\n" +
	"Select byemail for authors where email = :e\n"

func unresolvedHover(t *testing.T, needle string) string {
	t.Helper()
	root := voltProject(t, map[string]string{
		"go.mod":         "module proj\n",
		"db/schema.volt": unresolvedSchema,
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), unresolvedSchema)
	h := d.Hover(posOf(t, unresolvedSchema, needle, 0))
	if h == nil {
		t.Fatalf("no hover for the %s select", needle)
	}
	return h.Contents.(protocol.MarkupContent).Value
}

func TestSelectHoverUnresolved(t *testing.T) {
	for _, tc := range []struct{ needle, head string }{
		{"ranked", "Select ranked for authors"},
		{"shown", "Select shown (rank, email) for authors"},
	} {
		md := unresolvedHover(t, tc.needle)
		for _, want := range []string{"```volt\n" + tc.head + "\n```\n", "unresolved: column rank has an unknown type (§columns)\n"} {
			if !strings.Contains(md, want) {
				t.Errorf("%s: hover missing %q:\n%s", tc.needle, want, md)
			}
		}
		for _, leak := range []string{"func (q *Queries)", "WHERE", "ORDER BY", "binds:"} {
			if strings.Contains(md, leak) {
				t.Errorf("%s: an unresolved select's hover shows %q, which the dropped terms make wrong:\n%s", tc.needle, leak, md)
			}
		}
	}
}

func TestSelectHoverSpellsUnknownFieldType(t *testing.T) {
	for _, tc := range []struct{ needle, row string }{
		{"byemail", "type Author struct"},
		{"shown", "type Shown struct"},
	} {
		md := unresolvedHover(t, tc.needle)
		if !strings.Contains(md, tc.row) || !strings.Contains(md, "\tRank int32 // unknown type (§columns)\n") {
			t.Errorf("%s: hover does not spell the unknown field's declared type:\n%s", tc.needle, md)
		}
		for _, line := range strings.Split(md, "\n") {
			if f := strings.Fields(line); len(f) == 1 && strings.HasPrefix(line, "\t") {
				t.Errorf("%s: field %q printed with an empty Go type:\n%s", tc.needle, f[0], md)
			}
		}
	}
}

func TestSelectHoverNullableUnknownFieldType(t *testing.T) {
	schema := "package db\n\nType Score int32\n\n" +
		"Table authors {\n\tid int [pk, increment]\n\temail string\n\trank int32 [null]\n\tscore Score [null]\n}\n\n" +
		"Select shown (rank, score, email) for authors\n" +
		"Select byemail for authors where email = :e\n"
	root := voltProject(t, map[string]string{
		"go.mod":         "module proj\n",
		"db/schema.volt": schema,
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), schema)
	for _, needle := range []string{"shown", "byemail"} {
		h := d.Hover(posOf(t, schema, needle, 0))
		if h == nil {
			t.Fatalf("no hover for the %s select", needle)
		}
		md := h.Contents.(protocol.MarkupContent).Value
		for _, want := range []string{"\tRank int32 // unknown type (§columns)\n", "\tScore Score // unknown type (§columns)\n"} {
			if !strings.Contains(md, want) {
				t.Errorf("%s: hover missing %q:\n%s", needle, want, md)
			}
		}
		if strings.Contains(md, "rt.Null[]") {
			t.Errorf("%s: a nullable column of unknown type printed as rt.Null[]:\n%s", needle, md)
		}
	}
}
