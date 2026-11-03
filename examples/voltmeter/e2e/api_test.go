package e2e

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/nao/rt"
)

var bg = context.Background()

// TestPagesAndPlugs proves the root redirects into the app with a 303,
// the embedded assets are served by volt.File, any answers every verb,
// and the plugs of the api pipeline run before those of the member
// pipeline a nested scope appends (spec §pipelines, §scopes).
func TestPagesAndPlugs(t *testing.T) {
	e := start(t)
	r := e.raw("GET", "/", "")
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != app.PathAsset("voltmeter.html") {
		t.Fatalf("GET / = %d to %q", r.StatusCode, r.Header.Get("Location"))
	}
	page := e.raw("GET", r.Header.Get("Location"), "")
	if page.StatusCode != 200 || !strings.HasPrefix(page.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(page.body), "<h1>Voltmeter</h1>") {
		t.Errorf("landing page = %d %q", page.StatusCode, page.Header.Get("Content-Type"))
	}
	if css := e.raw("GET", app.PathAsset("voltmeter.css"), ""); css.StatusCode != 200 || !strings.HasPrefix(css.Header.Get("Content-Type"), "text/css") {
		t.Errorf("stylesheet = %d %q", css.StatusCode, css.Header.Get("Content-Type"))
	}
	for _, asset := range []string{"voltmeter.html", "voltmeter.css"} {
		b := e.raw("GET", app.PathAsset(asset), "").body
		if strings.Contains(string(b), "http://") || strings.Contains(string(b), "https://") {
			t.Errorf("%s names an external URL", asset)
		}
	}
	if missing := e.raw("GET", app.PathAsset("nothing.txt"), ""); missing.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset = %d", missing.StatusCode)
	}

	for _, method := range []string{"GET", "POST", "PATCH", "DELETE", "PUT"} {
		var pong app.Pong
		r := e.rawJSON(method, app.PathPing(), "", &pong)
		if r.StatusCode != 200 || pong.Method != method || pong.At.IsZero() {
			t.Errorf("%s /ping = %d %+v", method, r.StatusCode, pong)
		}
	}
	resp, err := e.gob(0).Ping(bg)
	read(t, resp, err, 200, nil)

	api := e.raw("GET", app.PathAPIRuns(), "", "X-Request-ID", "req-7")
	if got := api.Header.Values("X-Pipeline"); !slices.Equal(got, []string{"api"}) || api.Header.Get("X-Request-Id") != "req-7" || api.Header.Get("Server") != "voltmeter" {
		t.Errorf("api pipeline: X-Pipeline %v, X-Request-Id %q, Server %q", got, api.Header.Get("X-Request-Id"), api.Header.Get("Server"))
	}
	me := e.raw("GET", app.PathAPIMeWho(), "", "X-User", "1")
	if got := me.Header.Values("X-Pipeline"); !slices.Equal(got, []string{"api", "member"}) || me.Header.Get("Vary") != "X-User" || me.Header.Get("X-Request-Id") == "" {
		t.Errorf("member pipeline: X-Pipeline %v, Vary %q, X-Request-Id %q", got, me.Header.Get("Vary"), me.Header.Get("X-Request-Id"))
	}
	opts := e.raw("OPTIONS", app.PathAPIRunsAllowed(), "")
	if opts.StatusCode != http.StatusNoContent || opts.Header.Get("Allow") != "GET, HEAD, OPTIONS" {
		t.Errorf("OPTIONS /api/runs = %d %q", opts.StatusCode, opts.Header.Get("Allow"))
	}
}

// TestSignInAndScopeLoads proves the struct body of the sign-in, the
// cookie it sets, the Rescue that turns an unknown email into a 401, the
// scope load that halts with 401 before anything beneath runs, and the
// statuses of a body that does not decode (spec §request-bodies,
// §scope-loads, §formats).
func TestSignInAndScopeLoads(t *testing.T) {
	e := start(t)
	resp, err := e.gob(0).SignIn(bg, app.SignIn{Email: "grace@voltmeter.test"})
	var grace db.User
	r := read(t, resp, err, 200, &grace)
	if grace.ID != 2 || grace.Name != "Grace" {
		t.Fatalf("signed in as %+v", grace)
	}
	var cookie *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name == app.UserCookie {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value != "2" || !cookie.HttpOnly {
		t.Fatalf("sign-in cookie = %+v", cookie)
	}
	var who db.User
	if r := e.rawJSON("GET", app.PathAPIMeWho(), "", &who, "Cookie", cookie.String()); r.StatusCode != 200 || who.ID != 2 {
		t.Errorf("the cookie signs in: %d %+v", r.StatusCode, who)
	}
	if r := e.rawJSON("GET", app.PathAPIMeWho(), "", &who, "X-User", "3"); r.StatusCode != 200 || who.Name != "Ken" {
		t.Errorf("X-User signs in: %d %+v", r.StatusCode, who)
	}

	unknown := e.raw("POST", app.PathSignIn(), `{"email": "nobody@voltmeter.test"}`)
	if unknown.StatusCode != http.StatusUnauthorized || unknown.problem(t).Message != "no member signs in as nobody@voltmeter.test" {
		t.Errorf("an unknown email = %d %s", unknown.StatusCode, unknown.body)
	}
	shape := e.raw("POST", app.PathSignIn(), `{"email": "grace"}`)
	if p := shape.problem(t); shape.StatusCode != 422 || len(p.Details) != 1 || p.Details[0].Check != "EmailShape" || p.Details[0].Columns[0] != "email" {
		t.Errorf("a malformed email = %d %+v", shape.StatusCode, p)
	}
	extra := e.raw("POST", app.PathSignIn(), `{"email": "grace@voltmeter.test", "password": "x"}`)
	if p := extra.problem(t); extra.StatusCode != 400 || len(p.Details) != 1 || p.Details[0].Check != "unknown_field" || p.Details[0].Columns[0] != "password" {
		t.Errorf("an undeclared field = %d %+v", extra.StatusCode, p)
	}
	if r := e.raw("POST", app.PathSignIn(), `{"email": "grace@voltmeter.test"}`, "Content-Type", "text/plain"); r.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain = %d", r.StatusCode)
	}
	if r := e.raw("POST", app.PathSignIn(), `{"email": `); r.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON = %d", r.StatusCode)
	}
	if r := e.raw("POST", app.PathSignIn(), ""); r.StatusCode != http.StatusBadRequest {
		t.Errorf("an empty body = %d", r.StatusCode)
	}

	for _, header := range [][]string{nil, {"X-User", "ada"}, {"X-User", "0"}} {
		r := e.raw("GET", app.PathAPIMeDashboards(), "", header...)
		if r.StatusCode != http.StatusUnauthorized || r.Header.Get("X-Error-Handler") != "" || !strings.HasPrefix(r.problem(t).Message, "sign in") {
			t.Errorf("%v: %d, handler %q: want the load's own 401", header, r.StatusCode, r.Header.Get("X-Error-Handler"))
		}
	}
	gone := e.raw("GET", app.PathAPIMeWho(), "", "X-User", "99")
	if gone.StatusCode != http.StatusNotFound || gone.Header.Get("X-Error-Handler") != "api" {
		t.Errorf("a member no row has = %d via %q; want the lookup's 404 through the error spine", gone.StatusCode, gone.Header.Get("X-Error-Handler"))
	}
}

// TestUploadsAndRuns proves the collector's upload end to end through
// the GOB client: the machine found or added, the tag added on first
// use, every phase and keystroke row stored, rt.All reading a run back,
// the query routes over joins, self joins and the datasets of the phase
// group, and a run's annotation, report, export and deletion.
func TestUploadsAndRuns(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	first := upload(t, c, sweep("ABCDEF12", []int{2, 4}, 1))
	if first.Run.ID != 1 || first.Run.Commit != "abcdef12" || first.Run.Tag.Valid || len(first.Breaches) != 0 {
		t.Fatalf("first upload = %+v", first)
	}
	u := sweep("0123abcd", []int{2, 4}, 1.5)
	u.Tag, u.Note, u.Baseline, u.Dirty = "nightly", "after the fold", first.Run.ID, true
	second := upload(t, c, u)
	if second.Run.ID != 2 || second.Run.Tag != rt.Some("nightly") || second.Run.BaselineID != rt.Some(1) || !second.Run.Dirty {
		t.Fatalf("second upload = %+v", second.Run)
	}
	machines, err := c.APIMachines(bg)
	if err != nil || len(machines) != 1 || machines[0].CPU != machine.CPU {
		t.Fatalf("machines = %+v, %v; the second upload must find the first's machine", machines, err)
	}
	if host, err := c.APIHost(bg, machines[0].ID); err != nil || host.Go != machine.Go {
		t.Errorf("host = %+v, %v", host, err)
	}
	if _, err := c.APIHost(bg, 99); !errors.Is(err, volt.ErrNotFound) {
		t.Errorf("a missing host = %v", err)
	}
	if tag, err := c.APITag(bg, "nightly"); err != nil || tag.Name != "nightly" {
		t.Errorf("the tag added on first use = %+v, %v", tag, err)
	}
	if fleet, err := c.APIMachineFleet(bg); err != nil || len(fleet) != 1 || fleet[0].RunCount != 2 {
		t.Errorf("fleet = %+v, %v", fleet, err)
	}

	resp, err := c.APIRun(bg, 2)
	var detail app.RunDetail
	read(t, resp, err, 200, &detail)
	if detail.Run.ID != 2 || len(detail.Phases) != 8 || len(detail.Keystrokes) != 2 || detail.Latency.N != 0 {
		t.Fatalf("detail = %+v", detail)
	}
	var phases []string
	for _, p := range detail.Phases {
		phases = append(phases, string(p.Phase)+"@"+strconv.Itoa(p.Tables))
	}
	if got := strings.Join(phases, " "); got != "load@2 load@4 check@2 check@4 vet@2 vet@4 generate@2 generate@4" {
		t.Errorf("phase order %s", got)
	}
	if p := detail.Phases[3]; p.WallMS != units.Millis(1.5*2*4) || p.OutputMB != 0 {
		t.Errorf("check@4 = %+v", p)
	}
	if p := detail.Phases[7]; p.OutputMB != 0.4 {
		t.Errorf("generate@4 output = %v", p.OutputMB)
	}
	missing := e.raw("GET", app.PathAPIRun(99), "")
	if missing.StatusCode != 404 || missing.Header.Get("X-Error-Handler") != "api" {
		t.Errorf("a missing run = %d via %q", missing.StatusCode, missing.Header.Get("X-Error-Handler"))
	}
	if r := e.raw("GET", "/api/runs/seven", ""); r.StatusCode != 404 {
		t.Errorf("a run id that is no int = %d", r.StatusCode)
	}

	listed, err := c.APIRunListed(bg)
	if err != nil || len(listed) != 2 || listed[0].ID != 2 || listed[0].TagName != rt.Some("nightly") || listed[1].TagName.Valid || listed[0].MachineCPU != machine.CPU {
		t.Errorf("listed = %+v, %v", listed, err)
	}
	if untagged, err := c.APIRunUntagged(bg); err != nil || len(untagged) != 1 || untagged[0].ID != 1 {
		t.Errorf("untagged = %+v, %v", untagged, err)
	}
	if succ, err := c.APIRunSuccessors(bg, 1); err != nil || len(succ) != 1 || succ[0].FollowersID != rt.Some(2) {
		t.Errorf("successors = %+v, %v", succ, err)
	}

	loads, err := c.APIPhaseMsLoadRows(bg, 1)
	checks, err2 := c.APIPhaseMsCheckRows(bg, 1)
	vets, err3 := c.APIPhaseMsVetRows(bg, 1)
	gens, err4 := c.APIPhaseMsGenerateRows(bg, 1)
	if err := errors.Join(err, err2, err3, err4); err != nil || len(loads) != 2 || len(checks) != 2 || len(vets) != 2 || len(gens) != 2 || gens[1].OutputMb != 0.4 {
		t.Errorf("dataset rows: %d %d %d %d, %v", len(loads), len(checks), len(vets), len(gens), err)
	}
	tl, err := c.APITrendMsLoadTrend(bg, 4)
	tc, err2 := c.APITrendMsCheckTrend(bg, 4)
	tv, err3 := c.APITrendMsVetTrend(bg, 4)
	tg, err4 := c.APITrendMsGenerateTrend(bg, 4)
	if err := errors.Join(err, err2, err3, err4); err != nil || len(tl) != 2 || tl[0].RunID != 1 || tl[1].WallMs != 6 || len(tc) != 2 || len(tv) != 2 || len(tg) != 2 {
		t.Errorf("trend: %+v, %v", tl, err)
	}
	dl, err := c.APIDensityMsLoadDensity(bg, 1)
	dc, err2 := c.APIDensityMsCheckDensity(bg, 1)
	dv, err3 := c.APIDensityMsVetDensity(bg, 1)
	if err := errors.Join(err, err2, err3); err != nil || len(dl) != 2 || dl[0].AllocsPerTable != 100 || len(dc) != 2 || len(dv) != 2 {
		t.Errorf("density: %+v, %v", dl, err)
	}
	if r := e.raw("GET", "/api/density/generate?run=1", ""); r.StatusCode != 404 {
		t.Errorf("the member a dataset excepts = %d", r.StatusCode)
	}
	if r := e.raw("GET", app.PathAPIPhaseMsLoadRows(), ""); r.StatusCode != 400 || !strings.Contains(r.problem(t).Message, "run") {
		t.Errorf("a dataset route without its parameter = %d %s", r.StatusCode, r.body)
	}

	resp, err = c.APIRunAnnotate(bg, 1, db.RunNew().NoteSet(rt.Some("first light")).TagSet(rt.Some("baseline")))
	var annotated db.Run
	read(t, resp, err, 200, &annotated)
	if annotated.Note != rt.Some("first light") || annotated.Tag != rt.Some("baseline") || annotated.Commit != "abcdef12" {
		t.Errorf("annotated = %+v", annotated)
	}
	bad := e.raw("PATCH", app.PathAPIRunAnnotate(1), `{"tag": "no-such-tag"}`)
	if bad.StatusCode != 422 || bad.problem(t).Details[0].Check != "foreign_key" {
		t.Errorf("a tag no row has = %d %s", bad.StatusCode, bad.body)
	}
	var kept db.Run
	if r := e.rawJSON("PATCH", app.PathAPIRunAnnotate(1), `{"commit": "ffff0000", "note": "kept"}`, &kept); r.StatusCode != 200 || kept.Commit != "abcdef12" || kept.Note != rt.Some("kept") {
		t.Errorf("a column only: leaves out is not cast = %d %s", r.StatusCode, r.body)
	}
	if r := e.raw("PATCH", app.PathAPIRunAnnotate(1), `{"comit": "ffff0000"}`); r.StatusCode != 422 || r.problem(t).Details[0].Check != "unknown_field" {
		t.Errorf("a key that names no column = %d %s", r.StatusCode, r.body)
	}
	if r := e.raw("PATCH", app.PathAPIRunAnnotate(99), `{"note": "x"}`); r.StatusCode != 404 {
		t.Errorf("annotating a missing run = %d", r.StatusCode)
	}

	resp, err = c.APISamplesRecord(bg, 1, app.SampleBatch{Samples: []app.SampleTimed{
		{Route: "GET /api/runs", Status: 200, MS: 1.5, At: 1000},
		{Route: "GET /api/runs", Status: 200, MS: 2.5, At: 1001},
		{Route: "GET /api/runs/{id}", Status: 404, MS: 0.5, At: 1002},
	}})
	var stored app.Stored
	read(t, resp, err, http.StatusCreated, &stored)
	if stored.Written != 3 {
		t.Errorf("samples stored = %+v", stored)
	}
	if lat, err := c.APISampleLatency(bg, 1); err != nil || lat.N != 3 || lat.Worst != rt.Some(2.5) || lat.Best != rt.Some(0.5) {
		t.Errorf("latency = %+v, %v", lat, err)
	}
	if per, err := c.APISamplePerRoute(bg, 1); err != nil || len(per) != 2 || per[0].Route != "GET /api/runs" || per[0].N != 2 {
		t.Errorf("per route = %+v, %v", per, err)
	}
	if fails, err := c.APISampleFailures(bg, 1); err != nil || len(fails) != 1 || fails[0].Status != 404 {
		t.Errorf("failures = %+v, %v", fails, err)
	}
	if r := e.raw("POST", app.PathAPISamplesRecord(99), `{"samples": [{"route": "GET /", "status": 200, "ms": 1, "at": 1}]}`); r.StatusCode != 422 || r.problem(t).Details[0].Check != "foreign_key" {
		t.Errorf("samples of a missing run = %d %s", r.StatusCode, r.body)
	}

	if r := e.raw("DELETE", app.PathAPIRun(2), ""); r.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE run 2 = %d", r.StatusCode)
	}
	if r := e.raw("DELETE", app.PathAPIRun(2), ""); r.StatusCode != http.StatusNotFound {
		t.Errorf("DELETE run 2 again = %d", r.StatusCode)
	}
	if rows, err := c.APIPhaseMsLoadRows(bg, 2); err != nil || len(rows) != 0 {
		t.Errorf("the deleted run's rows = %d, %v; the cascade removes them", len(rows), err)
	}
}

// TestReports proves the report scope's own error handler wins over the
// root's (spec §scopes), the HTML report and the HEAD probe beside it,
// and the CSV export written by volt.Stream.
func TestReports(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	up := upload(t, c, sweep("feed5eed", []int{2, 4, 8}, 1))
	resp, err := c.ReportShow(bg, up.Run.ID)
	r := read(t, resp, err, 200, nil)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(r.body), "Voltmeter run 1") || strings.Count(string(r.body), "<polyline") != 4 {
		t.Errorf("report = %q, %d lines", r.Header.Get("Content-Type"), strings.Count(string(r.body), "<polyline"))
	}
	resp, err = c.ReportShow(bg, 99)
	r = read(t, resp, err, 404, nil)
	if r.Header.Get("X-Error-Handler") != "report" || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(r.body), "404 Not Found") {
		t.Errorf("a missing report = %q via %q: %s", r.Header.Get("Content-Type"), r.Header.Get("X-Error-Handler"), r.body)
	}
	resp, err = c.ReportProbe(bg, up.Run.ID)
	read(t, resp, err, 200, nil)
	resp, err = c.ReportProbe(bg, 99)
	r = read(t, resp, err, 404, nil)
	if r.Header.Get("X-Error-Handler") != "report" || len(r.body) != 0 {
		t.Errorf("HEAD of a missing report via %q with %d bytes", r.Header.Get("X-Error-Handler"), len(r.body))
	}
	resp, err = c.ReportExport(bg, up.Run.ID)
	r = read(t, resp, err, 200, nil)
	lines := strings.Split(strings.TrimSpace(string(r.body)), "\n")
	if r.Header.Get("Content-Type") != "text/csv; charset=utf-8" || r.Header.Get("Content-Disposition") != `attachment; filename="voltmeter-run-1.csv"` || len(lines) != 13 || lines[0] != strings.Join(app.ReportCSVHeader, ",") || lines[1] != "load,2,2,1,1,200,0,3" {
		t.Errorf("export %q %q:\n%s", r.Header.Get("Content-Type"), r.Header.Get("Content-Disposition"), r.body)
	}
	resp, err = c.ReportExport(bg, 99)
	if r := read(t, resp, err, 404, nil); r.Header.Get("X-Error-Handler") != "report" {
		t.Errorf("a missing export via %q", r.Header.Get("X-Error-Handler"))
	}
}

// TestBudgetsBreachesAndTransactions proves the budget routes, a breach
// written in the upload's transaction with the budget's new state, and
// an upload that fails inside the transaction leaving nothing behind:
// no measurement, no breach, no budget change and no run, whose id is
// still never reused (spec §changesets, §request-bodies).
func TestBudgetsBreachesAndTransactions(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	var check db.Budget
	if r := e.rawJSON("POST", app.PathAPIBudgets(), `{"phase": "check", "tables": 2, "max_ms": 5}`, &check); r.StatusCode != http.StatusCreated || check.State != db.HealthOK {
		t.Fatalf("POST budget = %d %s", r.StatusCode, r.body)
	}
	dup := e.raw("POST", app.PathAPIBudgets(), `{"phase": "check", "tables": 2, "max_ms": 9}`)
	if dup.StatusCode != http.StatusConflict || dup.problem(t).Details[0].Check != "unique" {
		t.Errorf("a second budget of one phase and size = %d %s", dup.StatusCode, dup.body)
	}
	invalid := e.raw("POST", app.PathAPIBudgets(), `{"phase": "link", "tables": 0, "max_ms": 1}`)
	if p := invalid.problem(t); invalid.StatusCode != 422 || len(p.Details) != 2 || p.Details[0].Check != "PhaseKnown" || p.Details[1].Check != "BudgetSane" {
		t.Errorf("an invalid budget = %d %+v", invalid.StatusCode, p)
	}
	if r := e.raw("POST", app.PathAPIBudgets(), `{"phase": "vet", "tables": "two", "max_ms": 1}`); r.StatusCode != 422 || r.problem(t).Details[0].Check != "cast" {
		t.Errorf("a budget whose tables do not cast = %d %s", r.StatusCode, r.body)
	}

	under := upload(t, c, sweep("aaaa0001", []int{2}, 1))
	if len(under.Breaches) != 0 {
		t.Errorf("check@2 at 2 ms breached a 5 ms budget: %+v", under.Breaches)
	}
	over := upload(t, c, sweep("aaaa0002", []int{2}, 4))
	if len(over.Breaches) != 1 || over.Breaches[0].BudgetID != check.ID || over.Breaches[0].WallMs != 16 || over.Breaches[0].RunID != over.Run.ID {
		t.Fatalf("check@2 at 16 ms over a 5 ms budget: breaches %+v", over.Breaches)
	}
	resp, err := c.APIBudget(bg, check.ID)
	var detail app.BudgetDetail
	read(t, resp, err, 200, &detail)
	if detail.Budget.State != db.HealthBreached || len(detail.Breaches) != 1 {
		t.Errorf("budget after the breach = %+v", detail)
	}
	var breached []db.Budget
	if r := e.rawJSON("GET", app.PathAPIBudgets(volt.Query("state", "breached")), "", &breached); r.StatusCode != 200 || len(breached) != 1 {
		t.Errorf("breached budgets = %d %+v", r.StatusCode, breached)
	}
	if r := e.raw("GET", app.PathAPIBudget(99), ""); r.StatusCode != 404 {
		t.Errorf("a missing budget = %d", r.StatusCode)
	}

	broken := sweep("aaaa0003", []int{2}, 4)
	broken.Phases = append(broken.Phases, broken.Phases[1])
	resp, err = c.APIUpload(bg, broken)
	r := read(t, resp, err, http.StatusConflict, nil)
	if p := r.problem(t); p.Details[0].Check != "unique" {
		t.Errorf("a duplicated phase row = %+v", p)
	}
	resp, err = c.APIRuns(bg)
	var page app.RunPage
	read(t, resp, err, 200, &page)
	if page.Total != 2 {
		t.Errorf("runs after the failed upload = %d, want 2: the run is deleted again", page.Total)
	}
	resp, err = c.APIBudget(bg, check.ID)
	read(t, resp, err, 200, &detail)
	if len(detail.Breaches) != 1 {
		t.Errorf("breaches after the failed upload = %d, want 1: the transaction rolled back", len(detail.Breaches))
	}
	if rows, err := c.APIPhaseMsLoadRows(bg, 3); err != nil || len(rows) != 0 {
		t.Errorf("rows of the failed run = %d, %v", len(rows), err)
	}
	next := upload(t, c, sweep("aaaa0004", []int{2}, 1))
	if next.Run.ID != 4 {
		t.Errorf("the run after the failed one has id %d, want 4: ids are never reused", next.Run.ID)
	}

	resp, err = c.APIBudgetCeiling(bg, check.ID, db.BudgetNew().MaxMsSet(-1))
	r = read(t, resp, err, 422, nil)
	if p := r.problem(t); len(p.Details) != 1 || p.Details[0].Check != "BudgetSane" || !slices.Equal(p.Details[0].Columns, []string{"tables", "max_ms"}) {
		t.Errorf("a negative ceiling = %+v", p)
	}
	var same db.Budget
	if r := e.rawJSON("PATCH", app.PathAPIBudgetCeiling(check.ID), `{"phase": "vet"}`, &same); r.StatusCode != 200 || same.Phase != db.PhaseCheck || same.MaxMs != 5 {
		t.Errorf("a column only: leaves out is not cast = %d %s", r.StatusCode, r.body)
	}
	resp, err = c.APIBudgetCeiling(bg, check.ID, db.BudgetNew().MaxMsSet(100))
	var raised db.Budget
	read(t, resp, err, 200, &raised)
	if raised.MaxMs != 100 || raised.State != db.HealthOK || raised.Phase != db.PhaseCheck {
		t.Errorf("raised = %+v", raised)
	}
	if r := e.raw("PATCH", app.PathAPIBudgetCeiling(99), `{"max_ms": 3}`); r.StatusCode != 404 {
		t.Errorf("the ceiling of a missing budget = %d", r.StatusCode)
	}

	resp, err = c.APIBudgetCeiling(bg, check.ID, db.BudgetNew().MaxMsSet(1))
	read(t, resp, err, 200, nil)
	upload(t, c, sweep("aaaa0005", []int{2}, 4))
	resp, err = c.APIBudgetsReset(bg)
	var reset app.Reset
	read(t, resp, err, 200, &reset)
	if reset.Reset != 1 {
		t.Errorf("reset = %+v", reset)
	}
	resp, err = c.APIBudgetsReset(bg)
	var none app.Reset
	read(t, resp, err, 200, &none)
	if none.Reset != 0 {
		t.Errorf("a second reset = %+v", none)
	}
	var all []db.Budget
	if r := e.rawJSON("GET", app.PathAPIBudgets(), "", &all); r.StatusCode != 200 || len(all) != 1 || all[0].State != db.HealthOK {
		t.Errorf("budgets = %+v", all)
	}
}

// TestKeysetPaging proves the hand-written keyset paging of the run
// list never repeats or skips a run, filters by tag, agrees with offset
// paging, and refuses a malformed page (spec §predicates, After).
func TestKeysetPaging(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	var want []int
	for i := range 23 {
		u := app.Upload{Commit: "cafe" + strconv.Itoa(1000+i), Machine: machine}
		if i%3 == 0 {
			u.Tag = "third"
		}
		want = append([]int{upload(t, c, u).Run.ID}, want...)
	}
	page := func(opts ...volt.URLOption) app.RunPage {
		t.Helper()
		resp, err := c.Raw(bg, "GET", app.PathAPIRuns(opts...), nil)
		var p app.RunPage
		r := read(t, resp, err, 200, &p)
		if r.Header.Get("X-Total") != strconv.FormatInt(p.Total, 10) {
			t.Errorf("X-Total %q, body %d", r.Header.Get("X-Total"), p.Total)
		}
		return p
	}
	var got []int
	pages := 0
	for after := 0; ; pages++ {
		opts := []volt.URLOption{volt.Query("limit", "5")}
		if after > 0 {
			opts = append(opts, volt.Query("after", strconv.Itoa(after)))
		}
		p := page(opts...)
		if p.Total != 23 {
			t.Fatalf("total %d", p.Total)
		}
		for _, r := range p.Runs {
			got = append(got, r.ID)
		}
		if p.Next == 0 {
			break
		}
		after = p.Next
	}
	if !slices.Equal(got, want) || pages != 4 {
		t.Errorf("keyset pages (%d) gave %v, want %v", pages+1, got, want)
	}
	if p := page(volt.Query("limit", "5"), volt.Query("offset", "10")); len(p.Runs) != 5 || p.Runs[0].ID != want[10] {
		t.Errorf("offset page = %+v", p.Runs)
	}
	var tagged []int
	for after := 0; ; {
		opts := []volt.URLOption{volt.Query("limit", "3"), volt.Query("tag", "third")}
		if after > 0 {
			opts = append(opts, volt.Query("after", strconv.Itoa(after)))
		}
		p := page(opts...)
		if p.Total != 8 {
			t.Fatalf("tagged total %d", p.Total)
		}
		for _, r := range p.Runs {
			tagged = append(tagged, r.ID)
		}
		if p.Next == 0 {
			break
		}
		after = p.Next
	}
	if !slices.Equal(tagged, []int{22, 19, 16, 13, 10, 7, 4, 1}) {
		t.Errorf("tagged runs %v", tagged)
	}
	for _, q := range []string{"?limit=0", "?limit=x", "?after=x", "?after=1&offset=1"} {
		if r := e.raw("GET", "/api/runs"+q, ""); r.StatusCode != 400 {
			t.Errorf("%s = %d", q, r.StatusCode)
		}
	}
}

// TestTagsByDefaultHandlers proves resources [default]: every action a
// generated query route over the table's CRUD, its statuses and its
// changeset body (spec §default-handlers).
func TestTagsByDefaultHandlers(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	created, err := c.APICreateTag(bg, db.TagNew().SlugSet("release").NameSet("Release"))
	if err != nil || created.Slug != "release" {
		t.Fatalf("create = %+v, %v", created, err)
	}
	_, err = c.APICreateTag(bg, db.TagNew().SlugSet("release-2").NameSet("RELEASE"))
	var pe *volt.ProblemError
	if !errors.As(err, &pe) || pe.Status != http.StatusConflict || pe.Problem.Details[0].Check != "unique" {
		t.Errorf("a name differing only in case = %v; the expression index makes it a 409", err)
	}
	_, err = c.APICreateTag(bg, db.TagNew().SlugSet("empty"))
	if !errors.As(err, &pe) || pe.Status != 422 || pe.Problem.Details[0].Check != "required" {
		t.Errorf("a tag without a name = %v", err)
	}
	if updated, err := c.APIUpdateTag(bg, "release", db.TagNew().NameSet("Releases")); err != nil || updated.Name != "Releases" {
		t.Errorf("update = %+v, %v", updated, err)
	}
	if _, err := c.APIUpdateTag(bg, "absent", db.TagNew().NameSet("x")); !errors.Is(err, volt.ErrNotFound) {
		t.Errorf("update of a missing tag = %v", err)
	}
	if tags, err := c.APITags(bg); err != nil || len(tags) != 2 || tags[0].Slug != "baseline" {
		t.Errorf("tags = %+v, %v", tags, err)
	}
	if err := c.APIDeleteTag(bg, "release"); err != nil {
		t.Errorf("delete = %v", err)
	}
	if _, err := c.APITag(bg, "release"); !errors.Is(err, volt.ErrNotFound) {
		t.Errorf("get after delete = %v", err)
	}
	if r := e.raw("POST", app.PathAPITags(), `{"slug": "x", "name": "X"}`, "Content-Type", "text/plain"); r.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain to a query route = %d", r.StatusCode)
	}
	if r := e.raw("GET", app.PathAPIRunListed(), "", "Accept", "text/csv"); r.StatusCode != http.StatusNotAcceptable || !strings.Contains(r.problem(t).Message, "application/x-gob") {
		t.Errorf("Accept text/csv on a query route = %d %s", r.StatusCode, r.body)
	}
	if r := e.raw("GET", app.PathAPIRuns(), "", "Accept", "text/csv"); r.StatusCode != http.StatusNotAcceptable {
		t.Errorf("Accept text/csv on a controller route = %d", r.StatusCode)
	}
	if r := e.raw("POST", app.PathAPITags(), `{"slug": "x", "name": "X", "colour": "red"}`); r.StatusCode != 422 || r.problem(t).Details[0].Check != "unknown_field" {
		t.Errorf("an unknown key = %d %s", r.StatusCode, r.body)
	}
}

// TestThesis proves the thesis upload stores what it does not have yet
// in one transaction and the query routes read it back joined and
// matched.
func TestThesis(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	body := app.ThesisUpload{Commits: []app.ThesisCommit{
		{SHA: "1111", Subject: "Fold the migrations", Author: "Ada", At: at, Counts: []app.ThesisCount{{Fixture: "itest/blog", VoltLines: 80, HandLines: 120, GenLines: 3000}}},
		{SHA: "2222", Subject: "Add Voltmeter", Author: "Grace", At: at.Add(time.Hour), Counts: []app.ThesisCount{{Fixture: "itest/blog", VoltLines: 82, HandLines: 118, GenLines: 3050}, {Fixture: "examples/voltmeter", VoltLines: 400, HandLines: 900, GenLines: 12000}}},
	}}
	resp, err := c.APIThesisRecord(bg, body)
	var stored app.Stored
	read(t, resp, err, http.StatusCreated, &stored)
	if stored.Written != 5 || stored.Skipped != 0 {
		t.Errorf("first record = %+v", stored)
	}
	resp, err = c.APIThesisRecord(bg, body)
	var again app.Stored
	read(t, resp, err, http.StatusOK, &again)
	if again.Written != 0 || again.Skipped != 5 {
		t.Errorf("second record = %+v", again)
	}
	if resp, err := c.APIThesisRecord(bg, app.ThesisUpload{}); err == nil {
		read(t, resp, err, 422, nil)
	}
	rows, err := c.APILineCountThesis(bg, "itest/blog")
	if err != nil || len(rows) != 2 || rows[0].Sha != "1111" || rows[1].CommitSubject != "Add Voltmeter" || rows[1].GenLines != 3050 {
		t.Errorf("thesis = %+v, %v", rows, err)
	}
	if found, err := c.APICommitMatching(bg, "%Voltmeter%"); err != nil || len(found) != 1 || found[0].Author != "Grace" {
		t.Errorf("matching = %+v, %v", found, err)
	}
}

// TestDashboardsUnderNestedLoads proves the member scope's load reaches
// every route beneath it, the board scope's load nested under it, the
// changeset bodies narrowed by only:, and that one member cannot reach
// another's board.
func TestDashboardsUnderNestedLoads(t *testing.T) {
	e := start(t)
	ada, grace := e.json(1), e.gob(2)
	resp, err := ada.APIMeDashboardAdd(bg, db.DashboardNew().NameSet("Check trend"))
	var board db.Dashboard
	read(t, resp, err, http.StatusCreated, &board)
	if board.OwnerID != 1 || board.Name != "Check trend" {
		t.Fatalf("board = %+v", board)
	}
	resp, err = ada.APIMeDashboardAdd(bg, db.DashboardNew().NameSet("Check trend"))
	if r := read(t, resp, err, http.StatusConflict, nil); r.problem(t).Details[0].Check != "unique" {
		t.Errorf("a second board of one name: %s", r.body)
	}
	resp, err = ada.APIMeDashboardAdd(bg, db.DashboardNew())
	if r := read(t, resp, err, 422, nil); r.problem(t).Details[0].Check != "required" {
		t.Errorf("a board without a name: %s", r.body)
	}
	resp, err = ada.APIMeDashboardAdd(bg, db.DashboardNew().NameSet("Mine").OwnerIDSet(2))
	var mine db.Dashboard
	read(t, resp, err, http.StatusCreated, &mine)
	if mine.OwnerID != 1 {
		t.Errorf("only: (name) leaves the owner to the member's load, got owner %d", mine.OwnerID)
	}

	resp, err = ada.APIMeBoardWidgetAdd(bg, board.ID, db.WidgetNew().PositionSet(1).PhaseSet(db.PhaseCheck).TablesSet(4))
	var widget db.Widget
	read(t, resp, err, http.StatusCreated, &widget)
	if widget.DashboardID != board.ID {
		t.Errorf("widget = %+v", widget)
	}
	resp, err = ada.APIMeBoardWidgetAdd(bg, board.ID, db.WidgetNew().PositionSet(2).PhaseSet("link").TablesSet(4))
	if r := read(t, resp, err, 422, nil); r.problem(t).Details[0].Check != "PhaseKnown" {
		t.Errorf("a widget of no phase: %s", r.body)
	}
	resp, err = grace.APIMeBoardWidgetAdd(bg, board.ID, db.WidgetNew().PositionSet(2).PhaseSet(db.PhaseVet).TablesSet(2))
	if r := read(t, resp, err, 404, nil); r.problem(t).Message != "no such dashboard" {
		t.Errorf("a widget on another member's board: %s", r.body)
	}
	resp, err = ada.APIMeBoardWidgetAdd(bg, 99, db.WidgetNew().PositionSet(2).PhaseSet(db.PhaseVet).TablesSet(2))
	if r := read(t, resp, err, 404, nil); r.Header.Get("X-Error-Handler") != "api" {
		t.Errorf("a board no row has: the nested load's 404 through the spine, got %q", r.Header.Get("X-Error-Handler"))
	}
	if r := e.raw("POST", app.PathAPIMeBoardWidgetAdd(board.ID), `{"position": 1, "phase": "vet", "tables": 2}`); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("the outer load halts before the nested one: %d", r.StatusCode)
	}

	resp, err = ada.APIMeDashboards(bg)
	var boards []db.DashboardWithWidgets
	read(t, resp, err, 200, &boards)
	if len(boards) != 2 || boards[0].Name != "Check trend" || len(boards[0].Widgets) != 1 || len(boards[1].Widgets) != 0 {
		t.Errorf("ada's boards = %+v", boards)
	}
	resp, err = grace.APIMeDashboards(bg)
	boards = nil
	read(t, resp, err, 200, &boards)
	if len(boards) != 0 {
		t.Errorf("grace's boards = %+v", boards)
	}
	resp, err = ada.APIMeDashboard(bg, board.ID)
	var shown db.DashboardWithWidgets
	read(t, resp, err, 200, &shown)
	if shown.ID != board.ID || len(shown.Widgets) != 1 {
		t.Errorf("shown = %+v", shown)
	}
	resp, err = grace.APIMeDashboard(bg, board.ID)
	read(t, resp, err, 404, nil)

	resp, err = grace.APIMeBoardWidgetRemove(bg, board.ID, widget.ID)
	read(t, resp, err, 404, nil)
	resp, err = ada.APIMeBoardWidgetRemove(bg, board.ID, widget.ID)
	read(t, resp, err, http.StatusNoContent, nil)
	resp, err = ada.APIMeBoardWidgetRemove(bg, board.ID, widget.ID)
	if r := read(t, resp, err, 404, nil); r.problem(t).Message != "no such widget" {
		t.Errorf("removing a removed widget: %s", r.body)
	}
}

// TestLimits proves the timeout bounds a route's effects with a 504
// through the error spine, the probe answers within it, and a body over
// MaxBodyBytes is a 413 before the handler runs (spec §route-loads,
// §formats).
func TestLimits(t *testing.T) {
	e := start(t)
	c := e.gob(0)
	resp, err := c.Raw(bg, "GET", app.PathAPIProbe(volt.Query("steps", "1000")), nil)
	var counted app.Counted
	read(t, resp, err, 200, &counted)
	if counted.Steps != 1000 || counted.Rows != 3 {
		t.Errorf("probe = %+v; the three seeded members", counted)
	}
	began := time.Now()
	slow := e.raw("GET", app.PathAPIProbe(volt.Query("steps", "4000000000")), "")
	if slow.StatusCode != http.StatusGatewayTimeout || slow.Header.Get("X-Error-Handler") != "api" {
		t.Errorf("a probe past the timeout = %d via %q: %s", slow.StatusCode, slow.Header.Get("X-Error-Handler"), slow.body)
	}
	if took := time.Since(began); took < time.Second || took > 10*time.Second {
		t.Errorf("the timed-out probe took %v; the route's timeout is one second", took)
	}
	if r := e.raw("GET", app.PathAPIProbe(), ""); r.StatusCode != 400 || r.problem(t).Message != "missing query parameter steps" {
		t.Errorf("a probe without steps = %d %s", r.StatusCode, r.body)
	}
	big := `{"commit": "` + strings.Repeat("a", int(volt.MaxBodyBytes)) + `"}`
	if r := e.raw("POST", app.PathAPIUpload(), big); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over the limit = %d", r.StatusCode)
	}
	if r := e.raw("POST", app.PathAPIUpload(), `{"commit": "zz"}`); r.StatusCode != 422 || r.problem(t).Details[0].Columns[0] != "commit" {
		t.Errorf("an upload of no commit = %d %s", r.StatusCode, r.body)
	}
}
