package app

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

// Reports renders a run for people: a printable page and a CSV export
// (spec §requests-and-responses, volt.HTML and volt.Stream). The routes
// load the run and its phase rows, so the actions only render.
type Reports struct{}

// Probe answers HEAD for a report without rendering it: whether the run
// exists.
func (Reports) Probe(r volt.Request, id int) volt.Response {
	return volt.Then(db.RunExists(db.RunID.Eq(id)), func(ok bool) volt.Response {
		if !ok {
			return volt.Fail(rt.ErrNotFound)
		}
		return volt.HTML(nil)
	})
}

// Show is the run's report page.
func (Reports) Show(r volt.Request, id int, run db.Run, loading []db.MsLoad, checking []db.MsCheck, vetting []db.MsVet, generating []db.MsGenerate) volt.Response {
	return volt.HTML(ReportHTML(run, wire.PhaseRowsOf(loading, checking, vetting, generating)))
}

// Export is the run's phase rows as CSV, written by a stream, the one
// response that is not a value.
func (Reports) Export(r volt.Request, id int, run db.Run, loading []db.MsLoad, checking []db.MsCheck, vetting []db.MsVet, generating []db.MsGenerate) volt.Response {
	rows := wire.PhaseRowsOf(loading, checking, vetting, generating)
	return volt.Stream(func(w io.Writer) error {
		return ReportCSV(w, rows)
	}).WithHeader("Content-Type", "text/csv; charset=utf-8").
		WithHeader("Content-Disposition", fmt.Sprintf(`attachment; filename="voltmeter-run-%d.csv"`, run.ID))
}

// ReportCSVHeader is the first record of every export.
var ReportCSVHeader = []string{"phase", "tables", "wall_ms", "alloc_mb", "heap_mb", "allocs", "output_mb", "source_kb"}

// ReportCSV writes the header and one record per phase row.
func ReportCSV(w io.Writer, rows []wire.PhaseRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(ReportCSVHeader); err != nil {
		return err
	}
	for _, p := range rows {
		rec := []string{
			string(p.Phase),
			strconv.Itoa(p.Tables),
			num(float64(p.WallMS)),
			num(p.AllocMB),
			num(p.HeapMB),
			strconv.Itoa(p.Allocs),
			num(p.OutputMB),
			num(p.SourceKB),
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

const reportStyle = `
.report {
  color-scheme: light;
  --surface: #fcfcfb;
  --ink: #0b0b0b;
  --ink-2: #52514e;
  --line: #deddd8;
  --series-1: #2a78d6;
  --series-2: #eb6834;
  --series-3: #1baf7a;
  --series-4: #eda100;
}
@media (prefers-color-scheme: dark) {
  :root:where(:not([data-theme="light"])) .report {
    color-scheme: dark;
    --surface: #1a1a19;
    --ink: #f0efec;
    --ink-2: #c3c2b7;
    --line: #3a3936;
    --series-1: #3987e5;
    --series-2: #d95926;
    --series-3: #199e70;
    --series-4: #c98500;
  }
}
:root[data-theme="dark"] .report {
  color-scheme: dark;
  --surface: #1a1a19;
  --ink: #f0efec;
  --ink-2: #c3c2b7;
  --line: #3a3936;
  --series-1: #3987e5;
  --series-2: #d95926;
  --series-3: #199e70;
  --series-4: #c98500;
}
body { margin: 0; background: #fcfcfb; }
@media (prefers-color-scheme: dark) { body { background: #1a1a19; } }
.report {
  background: var(--surface);
  color: var(--ink);
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  max-width: 960px;
  margin: 0 auto;
  padding: 24px 16px;
  line-height: 1.5;
}
.report h1, .report h2 { font-weight: 600; margin: 0 0 8px; }
.report p, .report dt { color: var(--ink-2); }
.report dl { display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px; }
.report dd { margin: 0; }
.report table { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
.report th, .report td { text-align: right; padding: 4px 8px; border-bottom: 1px solid var(--line); }
.report th:first-child, .report td:first-child { text-align: left; }
.report th { color: var(--ink-2); font-weight: 500; }
.report svg { width: 100%; height: auto; }
.report .axis { stroke: var(--line); }
.report .tick { fill: var(--ink-2); font-size: 12px; }
.report .label { fill: var(--ink); font-size: 12px; }
.report .legend { display: flex; flex-wrap: wrap; gap: 16px; margin: 8px 0; padding: 0; list-style: none; }
.report .legend span { display: inline-block; width: 12px; height: 2px; vertical-align: middle; margin-right: 6px; }
@media print { .report { max-width: none; } }
`

// ReportHTML is the self-contained report page of a run: what it
// measured on, a chart of wall time against table count per phase, and
// the table the chart draws. It uses no external resource and the
// system's sans-serif font.
func ReportHTML(run db.Run, rows []wire.PhaseRow) []byte {
	var b strings.Builder
	esc := html.EscapeString
	title := fmt.Sprintf("Voltmeter run %d", run.ID)
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n<style>%s</style>\n</head>\n<body>\n<main class=\"report\">\n", esc(title), reportStyle)
	fmt.Fprintf(&b, "<h1>%s</h1>\n<dl>\n", esc(title))
	commit := run.Commit.Short()
	if run.Dirty {
		commit += " (dirty)"
	}
	item := func(k, v string) { fmt.Fprintf(&b, "<dt>%s</dt><dd>%s</dd>\n", esc(k), esc(v)) }
	item("Commit", commit)
	item("Machine", strconv.Itoa(run.MachineID))
	if tag, ok := run.Tag.Get(); ok {
		item("Tag", tag)
	}
	if note, ok := run.Note.Get(); ok {
		item("Note", note)
	}
	item("Stored", run.CreatedAt.UTC().Format("2006-01-02 15:04 UTC"))
	b.WriteString("</dl>\n")
	if len(rows) == 0 {
		b.WriteString("<p>This run stored no phase row.</p>\n")
	} else {
		b.WriteString("<h2>Wall time by table count</h2>\n")
		reportChart(&b, rows)
		b.WriteString("<h2>Phase rows</h2>\n<table>\n<thead><tr>")
		for _, h := range []string{"Phase", "Tables", "Wall", "Alloc MiB", "Heap MiB", "Allocs", "Output MiB", "Source KiB"} {
			fmt.Fprintf(&b, "<th scope=\"col\">%s</th>", esc(h))
		}
		b.WriteString("</tr></thead>\n<tbody>\n")
		for _, p := range rows {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%d</td><td>%s</td><td>%.2f</td><td>%.2f</td><td>%d</td><td>%.2f</td><td>%.1f</td></tr>\n",
				esc(string(p.Phase)), p.Tables, esc(p.WallMS.String()), p.AllocMB, p.HeapMB, p.Allocs, p.OutputMB, p.SourceKB)
		}
		b.WriteString("</tbody>\n</table>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return []byte(b.String())
}

func reportChart(b *strings.Builder, rows []wire.PhaseRow) {
	const w, h, left, right, top, bottom = 720.0, 320.0, 64.0, 96.0, 16.0, 40.0
	sizeSet := map[int]bool{}
	peak := 0.0
	for _, p := range rows {
		sizeSet[p.Tables] = true
		peak = max(peak, float64(p.WallMS))
	}
	sizes := make([]int, 0, len(sizeSet))
	for s := range sizeSet {
		sizes = append(sizes, s)
	}
	sort.Ints(sizes)
	if peak == 0 {
		peak = 1
	}
	x := func(tables int) float64 {
		if len(sizes) == 1 || sizes[len(sizes)-1] == sizes[0] {
			return left + (w-left-right)/2
		}
		return left + (w-left-right)*float64(tables-sizes[0])/float64(sizes[len(sizes)-1]-sizes[0])
	}
	y := func(ms float64) float64 { return top + (h-top-bottom)*(1-ms/peak) }
	b.WriteString("<ul class=\"legend\">")
	for i, ph := range db.Phases {
		fmt.Fprintf(b, "<li><span style=\"background: var(--series-%d)\"></span>%s</li>", i+1, html.EscapeString(string(ph)))
	}
	b.WriteString("</ul>\n")
	fmt.Fprintf(b, "<svg viewBox=\"0 0 %g %g\" role=\"img\" aria-label=\"Wall time in milliseconds against table count, one line per phase\">\n", w, h)
	fmt.Fprintf(b, "<line class=\"axis\" x1=\"%g\" y1=\"%g\" x2=\"%g\" y2=\"%g\"/>\n", left, h-bottom, w-right, h-bottom)
	for _, frac := range []float64{0, 0.5, 1} {
		yy := y(peak * frac)
		fmt.Fprintf(b, "<line class=\"axis\" x1=\"%g\" y1=\"%.1f\" x2=\"%g\" y2=\"%.1f\" stroke-dasharray=\"2 4\"/>\n", left, yy, w-right, yy)
		fmt.Fprintf(b, "<text class=\"tick\" x=\"%g\" y=\"%.1f\" text-anchor=\"end\" dominant-baseline=\"middle\">%.1f ms</text>\n", left-8, yy, peak*frac)
	}
	for _, s := range sizes {
		fmt.Fprintf(b, "<text class=\"tick\" x=\"%.1f\" y=\"%g\" text-anchor=\"middle\">%d</text>\n", x(s), h-bottom+20, s)
	}
	fmt.Fprintf(b, "<text class=\"tick\" x=\"%g\" y=\"%g\" text-anchor=\"middle\">tables</text>\n", left+(w-left-right)/2, h-4)
	for i, ph := range db.Phases {
		var pts []wire.PhaseRow
		for _, p := range rows {
			if p.Phase == ph {
				pts = append(pts, p)
			}
		}
		if len(pts) == 0 {
			continue
		}
		sort.Slice(pts, func(a, c int) bool { return pts[a].Tables < pts[c].Tables })
		coords := make([]string, len(pts))
		for j, p := range pts {
			coords[j] = fmt.Sprintf("%.1f,%.1f", x(p.Tables), y(float64(p.WallMS)))
		}
		fmt.Fprintf(b, "<polyline fill=\"none\" stroke=\"var(--series-%d)\" stroke-width=\"2\" stroke-linejoin=\"round\" stroke-linecap=\"round\" points=\"%s\"/>\n", i+1, strings.Join(coords, " "))
		for _, p := range pts {
			fmt.Fprintf(b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"4\" fill=\"var(--series-%d)\" stroke=\"var(--surface)\" stroke-width=\"2\"><title>%s at %d tables: %s</title></circle>\n",
				x(p.Tables), y(float64(p.WallMS)), i+1, html.EscapeString(string(ph)), p.Tables, html.EscapeString(p.WallMS.String()))
		}
		last := pts[len(pts)-1]
		fmt.Fprintf(b, "<text class=\"label\" x=\"%.1f\" y=\"%.1f\" dominant-baseline=\"middle\">%s</text>\n", x(last.Tables)+10, y(float64(last.WallMS)), html.EscapeString(string(ph)))
	}
	b.WriteString("</svg>\n")
}

// ErrorPage is the page ReportErrors answers with: the status and the
// message, self-contained like the report.
func ErrorPage(status int, msg string) []byte {
	esc := html.EscapeString
	title := fmt.Sprintf("%d %s", status, http.StatusText(status))
	return []byte(fmt.Sprintf("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>%s</title>\n<style>%s</style>\n</head>\n<body>\n<main class=\"report\">\n<h1>%s</h1>\n<p>%s</p>\n</main>\n</body>\n</html>\n",
		esc(title), reportStyle, esc(title), esc(msg)))
}
