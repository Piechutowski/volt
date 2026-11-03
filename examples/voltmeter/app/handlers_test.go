package app_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

var clock = time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)

func request(method, target, route string, params ...string) volt.Request {
	return volt.RequestOf(httptest.NewRequest(method, target, nil), route, clock, nil, params...)
}

// TestControllersDeterministic calls every action twice with one
// request and zero-valued parameters through the generated helper, and
// finds none that answers two different values (spec
// §requests-and-responses).
func TestControllersDeterministic(t *testing.T) {
	r := request("GET", "/api/runs?limit=5&after=9", "GET /api/runs")
	if differ := app.ControllersDeterministic(app.NewControllers(), r); len(differ) != 0 {
		t.Errorf("actions that answered two values for one request: %v", differ)
	}
}

// TestHandlersDescribeTheirEffects proves the actions that reach the
// database return the continuation of the effect they name and nothing
// else, so each is tested with no database (spec
// §requests-and-responses): the same arguments describe the same effect
// and different ones a different effect.
func TestHandlersDescribeTheirEffects(t *testing.T) {
	r := request("GET", "/api/runs/7", "GET /api/runs/{id}", "id")
	cases := []struct {
		name      string
		got, want volt.Response
		other     volt.Response
	}{
		{
			"Runs.Show reads the run and its rows in parallel",
			app.Runs{}.Show(r, 7),
			volt.Then(rt.All(db.RunGet(7), db.MsLoadRows(7), db.MsCheckRows(7), db.MsVetRows(7), db.MsGenerateRows(7),
				db.KeystrokeQuery(db.KeystrokeRunID.Eq(7), db.KeystrokeOrderBy(db.KeystrokeTables.Asc())), db.SampleLatency(7)),
				func([]any) volt.Response { return volt.NoContent() }),
			app.Runs{}.Show(r, 8),
		},
		{
			"Runs.Delete deletes by a predicate and counts",
			app.Runs{}.Delete(r, 7),
			volt.Then(db.RunDeleteWhere(db.RunID.Eq(7)), func(int64) volt.Response { return volt.NoContent() }),
			app.Runs{}.Delete(r, 8),
		},
		{
			"Budgets.Reset updates every breached budget in one statement",
			app.Budgets{}.Reset(r),
			volt.Then(db.BudgetUpdateWhere(db.BudgetSet(db.BudgetState.Set(db.HealthOK)), db.BudgetState.Eq(db.HealthBreached)), func(int64) volt.Response { return volt.NoContent() }),
			volt.Then(db.BudgetUpdateWhere(db.BudgetSet(db.BudgetState.Set(db.HealthBreached)), db.BudgetState.Eq(db.HealthOK)), func(int64) volt.Response { return volt.NoContent() }),
		},
		{
			"Dashboards.Show reads the member's boards",
			app.Dashboards{}.Show(r, db.User{ID: 2}, 5),
			volt.Then(db.DashboardBoards(2), func([]db.DashboardWithWidgets) volt.Response { return volt.NoContent() }),
			app.Dashboards{}.Show(r, db.User{ID: 3}, 5),
		},
		{
			"Reports.Probe asks whether the run exists",
			app.Reports{}.Probe(r, 7),
			volt.Then(db.RunExists(db.RunID.Eq(7)), func(bool) volt.Response { return volt.NoContent() }),
			app.Reports{}.Probe(r, 8),
		},
	}
	for _, c := range cases {
		if !volt.Same(c.got, c.want) {
			t.Errorf("%s: the response is not the described continuation", c.name)
		}
		if volt.Same(c.got, c.other) {
			t.Errorf("%s: two different inputs describe one effect", c.name)
		}
	}
	probe := app.Probe{}.Count(request("GET", "/api/probe?steps=1000", "GET /api/probe"))
	if !volt.Same(probe, volt.Then(db.UserCount(rt.Raw[db.User](probeSQL(), 1000)), func(int64) volt.Response { return volt.NoContent() })) {
		t.Error("Probe.Count does not count the members while the database counts to steps")
	}
}

func probeSQL() string {
	return `(WITH RECURSIVE voltmeter_probe(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM voltmeter_probe WHERE x < ?) SELECT max(x) FROM voltmeter_probe) > 0`
}

// TestHandlersRefuseBeforeAnyEffect proves the actions that judge their
// input answer a done response without describing any effect: the
// status, the message and the itemized details are values a test
// compares (spec §requests-and-responses, §formats).
func TestHandlersRefuseBeforeAnyEffect(t *testing.T) {
	runs := func(query string) volt.Response {
		return app.Runs{}.Index(request("GET", "/api/runs"+query, "GET /api/runs"))
	}
	cases := []struct {
		name      string
		got, want volt.Response
	}{
		{"root redirects into the app", app.Home{}.Index(request("GET", "/", "GET /{$}")), volt.Redirect("/assets/voltmeter.html")},
		{"ping answers the method and the shim's clock", app.Home{}.Ping(request("DELETE", "/ping", "/ping")), volt.Render(wire.Pong{Method: "DELETE", At: clock})},
		{"options names the methods", app.Runs{}.Options(request("OPTIONS", "/api/runs", "OPTIONS /api/runs")), volt.NoContent().WithHeader("Allow", "GET, HEAD, OPTIONS")},
		{"a zero limit", runs("?limit=0"), volt.Reject(400, "limit runs from 1 to 100")},
		{"a limit over the cap", runs("?limit=101"), volt.Reject(400, "limit runs from 1 to 100")},
		{"after with offset", runs("?after=3&offset=2"), volt.Reject(400, "after and offset do not combine")},
		{"a malformed commit", app.Uploads{}.Create(request("POST", "/api/uploads", "POST /api/uploads"), app.Upload{Commit: "not hex"}),
			volt.Reject(422, "the upload is not valid").WithDetails([]rt.FieldError{{Check: "commit", Columns: []string{"commit"}, Message: commitErr("not hex")}})},
		{"an unknown phase", app.Uploads{}.Create(request("POST", "/api/uploads", "POST /api/uploads"), app.Upload{Commit: "abcd1234", Phases: []app.UploadPhase{{Phase: "link", Tables: 2}}}),
			volt.Reject(422, "the upload is not valid").WithDetails([]rt.FieldError{{Check: "PhaseKnown", Columns: []string{"phase"}, Message: "phase link is not one of the sweep's"}})},
		{"a sign-in that is no email", app.Session{}.SignIn(request("POST", "/sign_in", "POST /sign_in"), app.SignIn{Email: "ada"}),
			volt.Reject(422, "check the email").WithDetails([]rt.FieldError{{Check: "EmailShape", Columns: []string{"email"}, Message: `"ada" is not an email address`}})},
		{"a probe of no steps", app.Probe{}.Count(request("GET", "/api/probe?steps=0", "GET /api/probe")), volt.Reject(400, "steps must be at least 1")},
		{"a budget state that is none", app.Budgets{}.Index(request("GET", "/api/budgets?state=late", "GET /api/budgets")), volt.Reject(400, `"late" is not a budget state`)},
		{"a widget on another member's board", app.Widgets{}.Create(request("POST", "/api/me/dashboards/4/widgets", "POST /api/me/dashboards/{board}/widgets"), db.User{ID: 1}, db.Dashboard{ID: 4, OwnerID: 2}, 4, db.WidgetNew()),
			volt.Reject(404, "no such dashboard")},
		{"an empty sample batch", app.Samples{}.Create(request("POST", "/api/runs/1/samples", "POST /api/runs/{id}/samples"), 1, app.SampleBatch{}), volt.Reject(422, "the batch holds no sample")},
		{"an invalid ceiling", app.Budgets{}.Update(request("PATCH", "/api/budgets/1", "PATCH /api/budgets/{id}"), 1, db.Budget{ID: 1, Phase: db.PhaseLoad, Tables: 2, MaxMs: 5, State: db.HealthOK}.Changeset().MaxMsSet(-1).Validate()),
			volt.Reject(422, "check the ceiling").WithDetails([]rt.FieldError{{Check: "BudgetSane", Columns: []string{"tables", "max_ms"}, Message: "the ceiling must be above zero"}})},
	}
	for _, c := range cases {
		if !volt.Same(c.got, c.want) {
			t.Errorf("%s: got status %d, not the expected response", c.name, c.got.Status())
		}
	}
}

func commitErr(s string) string {
	_, err := units.CommitParse(s)
	return err.Error()
}

// TestUploadWrites proves the pure plan of an upload: one create per
// row, and for a row over a budget of its phase and size the budget's
// new state once and a breach per row, while a row under it or of
// another size writes nothing more.
func TestUploadWrites(t *testing.T) {
	run := db.Run{ID: 12}
	body := app.Upload{
		Phases: []app.UploadPhase{
			{Phase: db.PhaseLoad, Tables: 2, WallMS: 3},
			{Phase: db.PhaseCheck, Tables: 2, WallMS: 9},
			{Phase: db.PhaseCheck, Tables: 4, WallMS: 20},
			{Phase: db.PhaseGenerate, Tables: 2, WallMS: 1},
		},
		Keystrokes: []app.UploadKeystroke{{Tables: 2, MSPerOp: 0.5}},
	}
	budgets := []db.Budget{
		{ID: 1, Phase: db.PhaseCheck, Tables: 2, MaxMs: 5, State: db.HealthOK},
		{ID: 2, Phase: db.PhaseLoad, Tables: 2, MaxMs: 5, State: db.HealthOK},
		{ID: 3, Phase: db.PhaseCheck, Tables: 4, MaxMs: 10, State: db.HealthBreached},
	}
	steps, errs := app.UploadWrites(run, body, budgets)
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	var kinds []string
	for _, s := range steps {
		switch e := s.(type) {
		case rt.Write[db.MsLoad]:
			kinds = append(kinds, "load")
		case rt.Write[db.MsCheck]:
			kinds = append(kinds, "check")
		case rt.Write[db.MsGenerate]:
			kinds = append(kinds, "generate")
		case rt.Write[db.Budget]:
			kinds = append(kinds, "budget")
		case rt.Write[db.Breach]:
			kinds = append(kinds, "breach")
		case rt.Write[db.Keystroke]:
			kinds = append(kinds, "keystroke")
		default:
			t.Fatalf("unexpected step %T", e)
		}
	}
	want := "load check budget breach check breach generate keystroke"
	if got := strings.Join(kinds, " "); got != want {
		t.Errorf("steps = %s, want %s (the breached budget 3 keeps its state, so only budget 1 is updated)", got, want)
	}
	_, errs = app.UploadWrites(run, app.Upload{Phases: []app.UploadPhase{{Phase: db.PhaseVet, Tables: 2, WallMS: -1}}}, nil)
	if len(errs) != 1 || errs[0].Check != "MillisMeasured" {
		t.Errorf("a negative wall time: errors %v, want MillisMeasured", errs)
	}
}

// TestThesisWrites proves a thesis upload writes only the commits and
// counts not stored yet and counts the rest as skipped.
func TestThesisWrites(t *testing.T) {
	body := app.ThesisUpload{Commits: []app.ThesisCommit{
		{SHA: "aaaa", Counts: []app.ThesisCount{{Fixture: "itest/blog"}, {Fixture: "examples/voltmeter"}}},
		{SHA: "bbbb", Counts: []app.ThesisCount{{Fixture: "itest/blog"}}},
	}}
	steps, skipped := app.ThesisWrites(body, []db.Commit{{Sha: "aaaa"}}, []db.LineCount{{Sha: "aaaa", Fixture: "itest/blog"}})
	if len(steps) != 3 || skipped != 2 {
		t.Errorf("steps %d, skipped %d; want 3 writes (one count of aaaa, commit bbbb and its count) and 2 skipped", len(steps), skipped)
	}
}

// TestReportRendering proves the report is self-contained (no URL, a
// sans-serif system font, every phase charted and tabled), the error
// page escapes what it is handed, and the CSV export is exact.
func TestReportRendering(t *testing.T) {
	run := db.Run{ID: 9, Commit: "abcdef1234567890", Dirty: true, MachineID: 2, Tag: rt.Some("release"), Note: rt.Some("<b>fast</b>"), CreatedAt: clock}
	rows := wire.PhaseRowsOf(
		[]db.MsLoad{{RunID: 9, Tables: 2, WallMs: 1.5}, {RunID: 9, Tables: 4, WallMs: 2.5}},
		[]db.MsCheck{{RunID: 9, Tables: 2, WallMs: 3}},
		[]db.MsVet{{RunID: 9, Tables: 2, WallMs: 4}},
		[]db.MsGenerate{{RunID: 9, Tables: 2, WallMs: 5, OutputMb: 0.25}},
	)
	if len(rows) != 5 || rows[0].Phase != db.PhaseLoad || rows[2].Phase != db.PhaseCheck || rows[4].Phase != db.PhaseGenerate || rows[4].OutputMB != 0.25 {
		t.Fatalf("PhaseRowsOf = %+v", rows)
	}
	page := string(app.ReportHTML(run, rows))
	for _, want := range []string{"<title>Voltmeter run 9</title>", "abcdef1 (dirty)", "release", "&lt;b&gt;fast&lt;/b&gt;", "sans-serif", "<svg", "<polyline", ">generate</text>", "2.5 ms"} {
		if !strings.Contains(page, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	for _, banned := range []string{"http://", "https://", "url(", "@import", "<script", "<link", "monospace", "serif;"} {
		if strings.Contains(strings.ReplaceAll(page, "sans-serif", ""), banned) {
			t.Errorf("report carries %q; it must be self-contained in a sans-serif system font", banned)
		}
	}
	errPage := string(app.ErrorPage(http.StatusNotFound, "no <run>"))
	if !strings.Contains(errPage, "<h1>404 Not Found</h1>") || !strings.Contains(errPage, "no &lt;run&gt;") {
		t.Errorf("error page:\n%s", errPage)
	}
	var b bytes.Buffer
	if err := app.ReportCSV(&b, rows[:2]); err != nil {
		t.Fatal(err)
	}
	want := "phase,tables,wall_ms,alloc_mb,heap_mb,allocs,output_mb,source_kb\nload,2,1.5,0,0,0,0,0\nload,4,2.5,0,0,0,0,0\n"
	if b.String() != want {
		t.Errorf("csv:\n%s\nwant:\n%s", b.String(), want)
	}
}

// TestResponsesLeaveTheirReceiver proves a response is a value (spec
// §values-and-effects, rule 1; §requests-and-responses, rule 4):
// WithHeader, WithCookie and WithStatus return a new response and leave
// the one they were called on as it was.
func TestResponsesLeaveTheirReceiver(t *testing.T) {
	base := volt.Render(wire.Pong{Method: "GET"})
	headed := base.WithHeader("X-Probe", "1")
	cookied := base.WithCookie(http.Cookie{Name: app.UserCookie, Value: "1"})
	statused := base.WithStatus(http.StatusAccepted)
	if !volt.Same(base, volt.Render(wire.Pong{Method: "GET"})) {
		t.Error("WithHeader, WithCookie or WithStatus changed the response they were called on")
	}
	if volt.Same(base, headed) || volt.Same(base, cookied) || volt.Same(base, statused) || statused.Status() != http.StatusAccepted || base.Status() != http.StatusOK {
		t.Errorf("the derived responses do not differ from their receiver: %d, %d", base.Status(), statused.Status())
	}
}
