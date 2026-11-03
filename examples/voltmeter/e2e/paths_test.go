package e2e

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/collector"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/server"
)

// TestReverseURLsRoundTrip proves spec §route-names-and-reverse-urls by
// enumeration: every helper of the generated table builds a URL the
// router dispatches to that helper's own route, the count check makes
// the list total, and every timed call of the collector lands on the
// route its sample is labeled with. Escaping is checked on the wire: a
// tag slug holding "/", ".", ".." or a space reaches its own row, and a
// wildcard drops its empty elements.
func TestReverseURLsRoundTrip(t *testing.T) {
	e := envStart(t)
	h := server.Handler(e.db, e.broker)
	q := volt.Query
	built := map[string]struct{ method, url string }{
		"PathRoot":                        {"GET", app.PathRoot()},
		"PathPing":                        {"GET", app.PathPing()},
		"PathAsset":                       {"GET", app.PathAsset("voltmeter.html")},
		"PathSignIn":                      {"POST", app.PathSignIn()},
		"PathEvents":                      {"GET", app.PathEvents()},
		"PathAPIUpload":                   {"POST", app.PathAPIUpload()},
		"PathAPIThesisRecord":             {"POST", app.PathAPIThesisRecord()},
		"PathAPILineCountThesis":          {"GET", app.PathAPILineCountThesis(q("fixture", "itest/blog"))},
		"PathAPICommitMatching":           {"GET", app.PathAPICommitMatching(q("pattern", "%"))},
		"PathAPIProbe":                    {"GET", app.PathAPIProbe(q("steps", "1"))},
		"PathAPIRuns":                     {"GET", app.PathAPIRuns()},
		"PathAPIRun":                      {"GET", app.PathAPIRun(1)},
		"PathAPIRunsAllowed":              {"OPTIONS", app.PathAPIRunsAllowed()},
		"PathAPIRunListed":                {"GET", app.PathAPIRunListed()},
		"PathAPIRunUntagged":              {"GET", app.PathAPIRunUntagged()},
		"PathAPIRunSuccessors":            {"GET", app.PathAPIRunSuccessors(1)},
		"PathAPIRunAnnotate":              {"PATCH", app.PathAPIRunAnnotate(1)},
		"PathAPISamplesRecord":            {"POST", app.PathAPISamplesRecord(1)},
		"PathAPISampleLatency":            {"GET", app.PathAPISampleLatency(1)},
		"PathAPISamplePerRoute":           {"GET", app.PathAPISamplePerRoute(1)},
		"PathAPISampleFailures":           {"GET", app.PathAPISampleFailures(1)},
		"PathAPIMachines":                 {"GET", app.PathAPIMachines()},
		"PathAPIHost":                     {"GET", app.PathAPIHost(1)},
		"PathAPIMachineFleet":             {"GET", app.PathAPIMachineFleet()},
		"PathAPITags":                     {"GET", app.PathAPITags()},
		"PathAPITag":                      {"GET", app.PathAPITag("baseline")},
		"PathAPIUsers":                    {"GET", app.PathAPIUsers()},
		"PathAPIBudgets":                  {"GET", app.PathAPIBudgets()},
		"PathAPIBudget":                   {"GET", app.PathAPIBudget(1)},
		"PathAPIBudgetCeiling":            {"PATCH", app.PathAPIBudgetCeiling(1)},
		"PathAPIBudgetsReset":             {"POST", app.PathAPIBudgetsReset()},
		"PathAPIPhaseMsLoadRows":          {"GET", app.PathAPIPhaseMsLoadRows(q("run", "1"))},
		"PathAPIPhaseMsCheckRows":         {"GET", app.PathAPIPhaseMsCheckRows(q("run", "1"))},
		"PathAPIPhaseMsVetRows":           {"GET", app.PathAPIPhaseMsVetRows(q("run", "1"))},
		"PathAPIPhaseMsGenerateRows":      {"GET", app.PathAPIPhaseMsGenerateRows(q("run", "1"))},
		"PathAPITrendMsLoadTrend":         {"GET", app.PathAPITrendMsLoadTrend(q("tables", "2"))},
		"PathAPITrendMsCheckTrend":        {"GET", app.PathAPITrendMsCheckTrend(q("tables", "2"))},
		"PathAPITrendMsVetTrend":          {"GET", app.PathAPITrendMsVetTrend(q("tables", "2"))},
		"PathAPITrendMsGenerateTrend":     {"GET", app.PathAPITrendMsGenerateTrend(q("tables", "2"))},
		"PathAPIDensityMsLoadDensity":     {"GET", app.PathAPIDensityMsLoadDensity(q("run", "1"))},
		"PathAPIDensityMsCheckDensity":    {"GET", app.PathAPIDensityMsCheckDensity(q("run", "1"))},
		"PathAPIDensityMsVetDensity":      {"GET", app.PathAPIDensityMsVetDensity(q("run", "1"))},
		"PathAPISizedMsLoadSized":         {"GET", app.PathAPISizedMsLoadSized(1)},
		"PathAPISizedMsGenerateSized":     {"GET", app.PathAPISizedMsGenerateSized(1)},
		"PathAPIPerSizeMsLoadPerSize":     {"GET", app.PathAPIPerSizeMsLoadPerSize()},
		"PathAPIPerSizeMsCheckPerSize":    {"GET", app.PathAPIPerSizeMsCheckPerSize()},
		"PathAPIPerSizeMsVetPerSize":      {"GET", app.PathAPIPerSizeMsVetPerSize()},
		"PathAPIPerSizeMsGeneratePerSize": {"GET", app.PathAPIPerSizeMsGeneratePerSize()},
		"PathAPISizes":                    {"GET", app.PathAPISizes()},
		"PathAPIMeWho":                    {"GET", app.PathAPIMeWho()},
		"PathAPIMeBoardsOf":               {"GET", app.PathAPIMeBoardsOf(q("owner", "1"))},
		"PathAPIRunCompare":               {"GET", app.PathAPIRunCompare(1, q("baseline", "1"))},
		"PathAPIMeDashboards":             {"GET", app.PathAPIMeDashboards()},
		"PathAPIMeDashboard":              {"GET", app.PathAPIMeDashboard(1)},
		"PathAPIMeDashboardAdd":           {"POST", app.PathAPIMeDashboardAdd()},
		"PathAPIMeBoardWidgetAdd":         {"POST", app.PathAPIMeBoardWidgetAdd(1)},
		"PathAPIMeBoardWidgetRemove":      {"DELETE", app.PathAPIMeBoardWidgetRemove(1, 2)},
		"PathReportProbe":                 {"HEAD", app.PathReportProbe(1)},
		"PathReportShow":                  {"GET", app.PathReportShow(1)},
		"PathReportExport":                {"GET", app.PathReportExport(1)},
	}
	dispatched := func(method, url string) string {
		t.Helper()
		req := httptest.NewRequest(method, url, nil)
		req.Header.Set("X-User", "1")
		h.ServeHTTP(httptest.NewRecorder(), req)
		return req.Pattern
	}
	helpers := map[string]string{}
	for _, r := range app.Table {
		if r.Helper == "" {
			continue
		}
		pattern := r.Pattern
		if r.Method != "" {
			pattern = r.Method + " " + r.Pattern
		}
		helpers[r.Helper] = pattern
	}
	if len(helpers) != len(built) {
		t.Fatalf("the table has %d helpers and this test builds %d; add the new helper here", len(helpers), len(built))
	}
	for helper, want := range helpers {
		b, ok := built[helper]
		if !ok {
			t.Errorf("no round trip for %s", helper)
			continue
		}
		if helper == "PathEvents" {
			if b.url != "/events" {
				t.Errorf("PathEvents = %q", b.url)
			}
			continue
		}
		if got := dispatched(b.method, b.url); got != want {
			t.Errorf("%s builds %s %s, which dispatches to %q, want %q", helper, b.method, b.url, got, want)
		}
	}

	for _, call := range collector.ReadCalls(collector.Targets{Run: 1, Machine: 1, Tag: "baseline", Fixture: "itest/blog", Tables: 2, Board: 1, Budget: 1}) {
		got := dispatched(call.Method, call.Path)
		if !strings.Contains(got, " ") {
			got = call.Method + " " + got
		}
		if got != call.Route {
			t.Errorf("the collector times %s as %q, but it dispatches to %q", call.Path, call.Route, got)
		}
	}

	c := e.gob(0)
	for _, slug := range []string{"a/b", ".", "..", "two words", "50%"} {
		if _, err := c.APICreateTag(bg, db.TagNew().SlugSet(slug).NameSet("tag "+slug)); err != nil {
			t.Fatalf("create %q: %v", slug, err)
		}
		url := app.PathAPITag(slug)
		if strings.Contains(strings.TrimPrefix(url, "/api/tags/"), "/") || strings.HasSuffix(url, "/.") || strings.HasSuffix(url, "/..") {
			t.Errorf("PathAPITag(%q) = %q leaves a separator or dot segment unescaped", slug, url)
		}
		var got db.Tag
		if r := e.rawJSON("GET", url, "", &got); r.StatusCode != http.StatusOK || got.Slug != slug {
			t.Errorf("GET %s = %d %+v, want the tag %q", url, r.StatusCode, got, slug)
		}
	}
	for _, path := range []string{"voltmeter.html", "/voltmeter.html", "//voltmeter.html"} {
		url := app.PathAsset(path)
		if url != "/assets/voltmeter.html" {
			t.Errorf("PathAsset(%q) = %q, want the clean /assets/voltmeter.html", path, url)
		}
		if r := e.raw("GET", url, ""); r.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d", url, r.StatusCode)
		}
	}
	if got := app.PathAsset("x//y/"); !slices.Contains([]string{"/assets/x/y", "/assets/x/y/"}, got) {
		t.Errorf("PathAsset(x//y/) = %q, want its empty elements dropped", got)
	}
}
