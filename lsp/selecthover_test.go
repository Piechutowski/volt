// Select hover with projections (spec §V11.7): the hover shows the
// generated signatures and the actual output row structs — the shared
// type once for an explicit list, the per-member derivative for the
// star form.
package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const projSchema = "package db\n\n" +
	"Table page_views {\n\tid integer [pk, increment]\n\tsite varchar [not null]\n\tday integer [not null]\n\thits integer [not null, default: 0]\n}\n\n" +
	"Table link_clicks {\n\tid integer [pk, increment]\n\tsite varchar [not null]\n\tday integer [not null]\n\ttarget text [not null, default: '']\n}\n\n" +
	"Group metrics {\n\tpage_views\n\tlink_clicks\n}\n\n" +
	"Select summary (site, day) for metrics where day >= :from\n" +
	"Select public (* \\ site) for link_clicks\n"

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
		"PageViewSummary(ctx context.Context, from int32) ([]Summary, error)",
		"LinkClickSummary(ctx context.Context, from int32) ([]Summary, error)",
		"type Summary struct",
		"Site string",
		"Day int32",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
	// One shared type: the struct must be rendered exactly once.
	if strings.Count(md, "type Summary struct") != 1 {
		t.Errorf("shared row type rendered more than once:\n%s", md)
	}
	// Reading order: the struct first, then the SQL, then the signatures.
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
