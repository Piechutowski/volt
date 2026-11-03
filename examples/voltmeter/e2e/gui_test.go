package e2e

import (
	"context"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/collector"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/remote"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

type screen struct {
	t      *testing.T
	s      state.State
	r      *remote.Remote
	export string
	issued []state.Command
}

func (e *env) screen(t *testing.T) *screen {
	t.Helper()
	g := &screen{t: t, export: t.TempDir()}
	g.r = remote.New(e.srv.URL, g.export)
	g.do(state.Started{})
	return g
}

func (g *screen) do(in state.Intent) {
	g.t.Helper()
	queue := []state.Intent{in}
	for len(queue) > 0 {
		next, cmds := state.Update(g.s, queue[0])
		g.s, queue = next, queue[1:]
		for _, c := range cmds {
			g.issued = append(g.issued, c)
			queue = append(queue, state.Answer{Session: next.Session, Result: g.r.Perform(context.Background(), c)})
		}
	}
}

func (g *screen) edit(f state.Field, v string) { g.do(state.FieldEdit{Field: f, Value: v}) }

func (g *screen) commandsOf(in state.Intent) []state.Command {
	g.t.Helper()
	from := len(g.issued)
	g.do(in)
	return slices.Clone(g.issued[from:])
}

func (g *screen) signIn(email string) {
	g.t.Helper()
	g.do(state.MemberPick{Email: email})
	if g.s.User.Email != email || g.s.Screen != state.ScreenRuns {
		g.t.Fatalf("signing in as %s: user %+v on %v, notice %+v", email, g.s.User, g.s.Screen, g.s.Notice)
	}
}

// TestGUISignInBrowseAndOpen drives the client's state through the
// screens a member uses against the live server: signing in by picking
// a member, a refused sign-in with its field error, keyset paging of
// the runs, tag create, rename and delete inline, a run opened with its
// chart, annotated and exported, the trend, the thesis, and a dashboard
// with nested widgets (spec §generated-client, §event-routes).
func TestGUISignInBrowseAndOpen(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	for i := range 20 {
		runUpload(t, c, sweep("cafe"+strings.Repeat("0", 3)+string(rune('a'+i%6))+string(rune('0'+i/6)), []int{2, 4}, float64(i+1)))
	}

	g := e.screen(t)
	if g.s.Screen != state.ScreenSignIn || len(g.s.Members) != 3 || g.s.Members[0].Name != "Ada" {
		t.Fatalf("the sign-in screen = %v with members %+v", g.s.Screen, g.s.Members)
	}
	g.do(state.MemberPick{Email: "nobody@voltmeter.test"})
	if g.s.User.ID != 0 || g.s.Errors[state.FieldEmail] != "no member signs in as nobody@voltmeter.test" || g.s.Notice.Text != "" {
		t.Errorf("an unknown member: user %+v, email error %q, notice %+v; want the server's message under the field alone", g.s.User, g.s.Errors[state.FieldEmail], g.s.Notice)
	}
	g.do(state.MemberPick{Email: "ada"})
	if got := g.s.Errors[state.FieldEmail]; !strings.Contains(got, "not an email address") {
		t.Errorf("a malformed email shows %q under the field", got)
	}
	g.do(state.ScreenShow{Screen: state.ScreenBudgets})
	if g.s.Screen != state.ScreenSignIn {
		t.Errorf("a member signed out reached %v", g.s.Screen)
	}
	g.signIn("ada@voltmeter.test")
	if g.s.Errors[state.FieldEmail] != "" {
		t.Errorf("the email error outlived the sign-in: %q", g.s.Errors[state.FieldEmail])
	}

	var ids []int
	for page := 0; ; page++ {
		if !g.s.Runs.Loaded || g.s.Runs.Page.Total != 20 {
			t.Fatalf("page %d: %+v", page, g.s.Runs)
		}
		for _, r := range g.s.Runs.Page.Runs {
			ids = append(ids, r.ID)
		}
		if g.s.Runs.Page.Next == 0 {
			break
		}
		g.do(state.RunsNext{})
	}
	want := make([]int, 0, 20)
	for id := 20; id >= 1; id-- {
		want = append(want, id)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("paged runs = %v, want %v", ids, want)
	}
	g.do(state.RunsPrev{})
	if g.s.Runs.After != 0 || len(g.s.Runs.Page.Runs) != state.RunsLimit || g.s.Runs.Page.Runs[0].ID != 20 {
		t.Errorf("back to the first page: after %d, %d runs", g.s.Runs.After, len(g.s.Runs.Page.Runs))
	}
	if cmds := g.commandsOf(state.RunsPrev{}); len(cmds) != 0 {
		t.Errorf("Previous on the first page asked %v", cmds)
	}

	g.edit(state.FieldTagSlug, "nightly")
	g.edit(state.FieldTagName, "Nightly")
	g.do(state.TagAdd{})
	if !slices.ContainsFunc(g.s.Tags, func(t db.Tag) bool { return t.Slug == "nightly" && t.Name == "Nightly" }) || g.s.Fields[state.FieldTagSlug] != "" {
		t.Fatalf("tags after adding nightly = %+v, fields %v", g.s.Tags, g.s.Fields)
	}
	g.edit(state.FieldTagSlug, "nightly")
	g.edit(state.FieldTagName, "Again")
	g.do(state.TagAdd{})
	if !strings.HasPrefix(g.s.Notice.Text, "409") || !g.s.Notice.Failed {
		t.Errorf("a second nightly: notice %+v", g.s.Notice)
	}
	g.edit(state.FieldTagSlug, "weekly")
	g.edit(state.FieldTagName, "")
	g.do(state.TagAdd{})
	if g.s.Errors[state.FieldTagName] == "" || g.s.Errors[state.FieldTagSlug] != "" {
		t.Errorf("a tag without a name: errors %v", g.s.Errors)
	}
	g.edit(state.FieldTagSlug, "nightly")
	g.edit(state.FieldTagName, "Nightly builds")
	g.do(state.TagRename{})
	if !slices.ContainsFunc(g.s.Tags, func(t db.Tag) bool { return t.Slug == "nightly" && t.Name == "Nightly builds" }) || len(g.s.Errors) != 0 {
		t.Errorf("tags after the rename = %+v, errors %v", g.s.Tags, g.s.Errors)
	}

	g.do(state.RunOpen{ID: 20})
	d := g.s.Run.Detail
	if g.s.Screen != state.ScreenRun || !g.s.Run.Loaded || d.Run.ID != 20 || len(d.Phases) != 8 {
		t.Fatalf("run 20 opened as %v: %+v", g.s.Screen, g.s.Run)
	}
	chart := state.RunChart(d)
	if len(chart.Series) != 4 || chart.Series[3].Name != "generate" || chart.Series[3].Points[1] != (state.Point{X: 4, Y: 20 * 4 * 4}) || len(chart.XTicks) != 2 {
		t.Errorf("the run chart = %+v", chart)
	}
	g.edit(state.FieldRunTag, "no-such-tag")
	g.do(state.RunAnnotate{})
	if g.s.Errors[state.FieldRunTag] == "" || g.s.Run.Detail.Run.Tag.Valid {
		t.Errorf("a tag no row has: errors %v, notice %+v, run %+v", g.s.Errors, g.s.Notice, g.s.Run.Detail.Run)
	}
	g.edit(state.FieldRunTag, "nightly")
	g.edit(state.FieldRunNote, "seen from the client")
	resp, err := c.APISamplesRecord(bg, 20, app.SampleBatch{Samples: []app.SampleTimed{{Route: "GET /ping", Status: 200, MS: 0.3, At: 1}}})
	replyRead(t, resp, err, http.StatusCreated, nil)
	g.do(state.Changed{Method: "POST", Handler: "Samples.Create", Path: app.PathAPISamplesRecord(20), Status: 201})
	if g.s.Fields[state.FieldRunNote] != "seen from the client" || g.s.Run.Detail.Latency.N != 1 {
		t.Errorf("a live refetch of the run while its note was edited left note %q, latency %+v", g.s.Fields[state.FieldRunNote], g.s.Run.Detail.Latency)
	}
	g.do(state.RunAnnotate{})
	if r := g.s.Run.Detail.Run; r.Tag.V != "nightly" || r.Note.V != "seen from the client" || len(g.s.Errors) != 0 {
		t.Errorf("annotated run = %+v, errors %v", r, g.s.Errors)
	}
	g.do(state.ScreenShow{Screen: state.ScreenRuns})
	g.do(state.RunsFilter{Tag: "nightly"})
	if p := g.s.Runs.Page; p.Total != 1 || len(p.Runs) != 1 || p.Runs[0].ID != 20 {
		t.Errorf("runs tagged nightly = %+v", p)
	}

	g.do(state.RunOpen{ID: 20})
	g.do(state.RunExport{})
	saved, err := os.ReadFile(g.s.Run.Export)
	if err != nil {
		t.Fatalf("the export: %v (notice %+v)", err, g.s.Notice)
	}
	resp, err = c.ReportExport(bg, 20)
	served := replyRead(t, resp, err, http.StatusOK, nil)
	if string(saved) != string(served.body) || !strings.HasPrefix(g.s.Run.Export, g.export) {
		t.Errorf("the saved CSV at %s differs from the served one:\n%s\n%s", g.s.Run.Export, saved, served.body)
	}

	g.do(state.ScreenShow{Screen: state.ScreenTrends})
	if g.s.Trend != (state.Spot{Phase: db.PhaseLoad, Tables: 2}) || !slices.Equal(g.s.Sizes(), []int{2, 4}) {
		t.Errorf("the trend opened at %+v with sizes %v", g.s.Trend, g.s.Sizes())
	}
	g.do(state.TrendPick{Spot: state.Spot{Phase: db.PhaseCheck, Tables: 4}})
	points := g.s.Trends[g.s.Trend]
	if len(points) != 20 || points[0].RunID != 1 || points[19].WallMS != 20*2*4 {
		t.Errorf("check at 4 across runs = %+v", points)
	}

	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	resp, err = c.APIThesisRecord(bg, app.ThesisUpload{Commits: []app.ThesisCommit{
		{SHA: "1111aaaa", Subject: "Fold the migrations", Author: "Ada", At: at, Counts: []app.ThesisCount{{Fixture: "itest/blog", VoltLines: 80, HandLines: 120, GenLines: 3000}}},
		{SHA: "2222bbbb", Subject: "Add Voltmeter", Author: "Grace", At: at.Add(time.Hour), Counts: []app.ThesisCount{{Fixture: "itest/blog", VoltLines: 82, HandLines: 118, GenLines: 3050}}},
	}})
	replyRead(t, resp, err, http.StatusCreated, nil)
	if !slices.Equal(state.Fixtures, collector.FixturesDefault) {
		t.Errorf("the thesis screen offers %v, the collector counts %v", state.Fixtures, collector.FixturesDefault)
	}
	g.do(state.ScreenShow{Screen: state.ScreenThesis})
	thesis := state.ThesisChart(g.s.Thesis.Rows)
	if g.s.Thesis.Fixture != "itest/blog" || len(g.s.Thesis.Rows) != 2 || len(thesis.Series) != 3 || thesis.Series[2].Points[1].Y != 3050 || thesis.XTicks[0].Label != "1111aaa" {
		t.Errorf("the thesis = %+v, chart %+v", g.s.Thesis, thesis)
	}
	g.do(state.ThesisPick{Fixture: "examples/voltmeter"})
	if !g.s.Thesis.Loaded || len(g.s.Thesis.Rows) != 0 {
		t.Errorf("a fixture without counts = %+v", g.s.Thesis)
	}

	g.do(state.ScreenShow{Screen: state.ScreenDashboards})
	if !g.s.Boards.Loaded || len(g.s.Boards.List) != 0 {
		t.Fatalf("Ada's dashboards = %+v", g.s.Boards)
	}
	g.edit(state.FieldBoardName, "Front end")
	g.do(state.BoardAdd{})
	g.do(state.BoardAdd{})
	if len(g.s.Boards.List) != 1 || g.s.Errors[state.FieldBoardName] == "" {
		t.Errorf("a second board of one name: boards %+v, errors %v", g.s.Boards.List, g.s.Errors)
	}
	board, ok := g.s.BoardSelected()
	if !ok || board.Name != "Front end" {
		t.Fatalf("the selected board = %+v", board)
	}
	g.edit(state.FieldWidgetTables, "4")
	g.do(state.WidgetAdd{})
	g.edit(state.FieldWidgetPhase, string(db.PhaseVet))
	g.edit(state.FieldWidgetTables, "2")
	g.do(state.WidgetAdd{})
	g.edit(state.FieldWidgetPhase, "link")
	g.do(state.WidgetAdd{})
	if got := g.s.Errors[state.FieldWidgetPhase]; !strings.Contains(got, "not a phase") {
		t.Errorf("a widget of no phase shows %q", got)
	}
	g.edit(state.FieldWidgetTables, "two")
	if cmds := g.commandsOf(state.WidgetAdd{}); len(cmds) != 0 || g.s.Errors[state.FieldWidgetTables] == "" {
		t.Errorf("a size that is no number asked %v, errors %v", cmds, g.s.Errors)
	}
	board, _ = g.s.BoardSelected()
	if len(board.Widgets) != 2 || board.Widgets[0].Phase != db.PhaseLoad || board.Widgets[1].Position != 2 {
		t.Fatalf("widgets = %+v", board.Widgets)
	}
	for _, w := range board.Widgets {
		if n := len(g.s.Trends[state.Spot{Phase: w.Phase, Tables: w.Tables}]); n != 20 {
			t.Errorf("widget %s at %d has %d points", w.Phase, w.Tables, n)
		}
	}
	g.do(state.WidgetRemove{ID: board.Widgets[0].ID})
	if board, _ = g.s.BoardSelected(); len(board.Widgets) != 1 || board.Widgets[0].Phase != db.PhaseVet {
		t.Errorf("widgets after the removal = %+v", board.Widgets)
	}

	grace := e.screen(t)
	grace.signIn("grace@voltmeter.test")
	grace.do(state.ScreenShow{Screen: state.ScreenDashboards})
	if len(grace.s.Boards.List) != 0 {
		t.Errorf("Grace sees Ada's boards: %+v", grace.s.Boards.List)
	}

	g.do(state.ScreenShow{Screen: state.ScreenRuns})
	g.edit(state.FieldTagSlug, "nightly")
	g.do(state.TagRemove{})
	if slices.ContainsFunc(g.s.Tags, func(t db.Tag) bool { return t.Slug == "nightly" }) || g.s.Runs.Tag != "" || g.s.Runs.Page.Total != 20 {
		t.Errorf("after deleting nightly: tags %+v, runs %+v", g.s.Tags, g.s.Runs)
	}
	g.do(state.RunOpen{ID: 20})
	if g.s.Run.Detail.Run.Tag.Valid {
		t.Errorf("run 20 kept a deleted tag: %+v", g.s.Run.Detail.Run)
	}

	g.do(state.SignOut{})
	if g.s.User.ID != 0 || g.s.Screen != state.ScreenSignIn || len(g.s.Members) != 3 {
		t.Errorf("signed out: %+v", g.s)
	}
	if res, ok := g.r.Perform(bg, state.BoardsFetch{}).(state.BoardsFetched); !ok || res.Err == nil || !strings.Contains(res.Err.Error(), "sign in") {
		t.Errorf("the dropped cookie still signs in: %+v", res)
	}
}

// TestGUIBudgetsLive proves a refused budget's field error lands under
// its field and clears when fixed, a value that is no number never
// reaches the server, a breach caused by another client reaches a
// second client's state live through the event stream, and so do a
// ceiling and a tag another member changed: the second member's open
// ceiling field and run form follow them, so a save neither reverts the
// ceiling nor names a deleted tag (spec §event-routes).
func TestGUIBudgetsLive(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	runUpload(t, c, sweep("beef0001", []int{2}, 1))

	ada := e.screen(t)
	ada.signIn("ada@voltmeter.test")
	grace := e.screen(t)
	grace.signIn("grace@voltmeter.test")
	grace.do(state.ScreenShow{Screen: state.ScreenBudgets})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	live := grace.r.Events(ctx)

	ada.do(state.ScreenShow{Screen: state.ScreenBudgets})
	if ada.s.Fields[state.FieldBudgetPhase] != string(db.PhaseLoad) {
		t.Errorf("the budget phase opens at %q", ada.s.Fields[state.FieldBudgetPhase])
	}
	ada.edit(state.FieldBudgetPhase, string(db.PhaseCheck))
	ada.edit(state.FieldBudgetTables, "0")
	ada.edit(state.FieldBudgetMax, "5")
	ada.do(state.BudgetAdd{})
	if got := ada.s.Errors[state.FieldBudgetTables]; !strings.Contains(got, "at least 1") || ada.s.Errors[state.FieldBudgetMax] == "" {
		t.Errorf("a budget at zero tables: errors %v", ada.s.Errors)
	}
	ada.edit(state.FieldBudgetTables, "2")
	ada.do(state.BudgetAdd{})
	b, ok := ada.s.BudgetSelected()
	if !ok || b.Phase != db.PhaseCheck || b.MaxMs != 5 || b.State != db.HealthOK || len(ada.s.Errors) != 0 {
		t.Fatalf("the added budget = %+v (errors %v, notice %+v)", b, ada.s.Errors, ada.s.Notice)
	}
	ada.edit(state.FieldBudgetTables, "2")
	ada.edit(state.FieldBudgetMax, "9")
	ada.do(state.BudgetAdd{})
	if !strings.HasPrefix(ada.s.Notice.Text, "409") || ada.s.Errors[state.FieldBudgetPhase] == "" || ada.s.Errors[state.FieldBudgetTables] == "" {
		t.Errorf("a second check budget at 2: notice %+v, errors %v", ada.s.Notice, ada.s.Errors)
	}

	if ada.s.Fields[state.FieldCeiling] != "5" {
		t.Errorf("the ceiling field holds %q", ada.s.Fields[state.FieldCeiling])
	}
	ada.edit(state.FieldCeiling, "-1")
	ada.do(state.BudgetCeilingSave{})
	if got := ada.s.Errors[state.FieldCeiling]; got != "the ceiling must be a finite number above zero" {
		t.Errorf("a negative ceiling shows %q under the field", got)
	}
	ada.edit(state.FieldCeiling, "fast")
	if cmds := ada.commandsOf(state.BudgetCeilingSave{}); len(cmds) != 0 || ada.s.Errors[state.FieldCeiling] == "" {
		t.Errorf("a ceiling that is no number asked %v", cmds)
	}
	ada.edit(state.FieldCeiling, "3")
	ada.do(state.BudgetCeilingSave{})
	if b, _ := ada.s.BudgetSelected(); b.MaxMs != 3 || ada.s.Errors[state.FieldCeiling] != "" {
		t.Errorf("the fixed ceiling = %+v, error %q", b, ada.s.Errors[state.FieldCeiling])
	}

	await := func(handler string) {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case ev, ok := <-live:
				if !ok {
					t.Fatal("the event stream closed")
				}
				in, known := state.ChangedOf(ev)
				if !known {
					continue
				}
				grace.do(in)
				if in.(state.Changed).Handler == handler {
					return
				}
			case <-timeout:
				t.Fatalf("no %s event reached the second client", handler)
			}
		}
	}
	await("Budgets.Update")
	if len(grace.s.Budgets.List) != 1 || grace.s.Budgets.List[0].MaxMs != 3 || grace.s.Budgets.Selected != grace.s.Budgets.List[0].ID {
		t.Fatalf("Grace's budgets = %+v", grace.s.Budgets)
	}
	if got := grace.s.Fields[state.FieldCeiling]; got != "3" {
		t.Errorf("Grace's ceiling field reads %q after Ada set 3; saving it would undo Ada's change", got)
	}

	over := runUpload(t, c, sweep("beef0002", []int{2}, 4))
	await("Uploads.Create")
	gb, _ := grace.s.BudgetSelected()
	breaches := grace.s.Budgets.Detail.Breaches
	if gb.State != db.HealthBreached || len(breaches) != 1 || breaches[0].RunID != over.Run.ID || breaches[0].WallMs != 16 {
		t.Errorf("Grace sees budget %+v with breaches %+v", gb, breaches)
	}
	if grace.s.Live != "a run was uploaded" || grace.s.Runs.Page.Total != 2 {
		t.Errorf("Grace's live line %q, runs %+v", grace.s.Live, grace.s.Runs.Page)
	}

	ada.do(state.BudgetsReset{})
	if ada.s.Notice.Text != "Reset 1 budget to ok" {
		t.Errorf("the reset says %q", ada.s.Notice.Text)
	}
	await("Budgets.Reset")
	if gb, _ := grace.s.BudgetSelected(); gb.State != db.HealthOK || len(grace.s.Budgets.Detail.Breaches) != 1 {
		t.Errorf("after the reset Grace sees %+v, breaches %+v", gb, grace.s.Budgets.Detail.Breaches)
	}

	if r := e.raw("POST", app.PathAPITags(), `{"slug": "sweep", "name": "Sweep"}`); r.StatusCode != http.StatusCreated {
		t.Fatalf("tag = %d %s", r.StatusCode, r.body)
	}
	await("db.TagCreate")
	if r := e.raw("PATCH", app.PathAPIRunAnnotate(over.Run.ID), `{"tag": "sweep"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("annotate = %d %s", r.StatusCode, r.body)
	}
	await("Runs.Annotate")
	grace.do(state.RunOpen{ID: over.Run.ID})
	if got := grace.s.Fields[state.FieldRunTag]; got != "sweep" {
		t.Fatalf("Grace's open run shows tag %q", got)
	}
	if r := e.raw("DELETE", app.PathAPITag("sweep"), ""); r.StatusCode != http.StatusNoContent {
		t.Fatalf("tag delete = %d %s", r.StatusCode, r.body)
	}
	await("db.TagDelete")
	if got := grace.s.Fields[state.FieldRunTag]; got != "" || grace.s.Run.Detail.Run.Tag.Valid {
		t.Errorf("after Ada deleted the tag Grace's open run still shows %q (%+v)", got, grace.s.Run.Detail.Run.Tag)
	}
	grace.edit(state.FieldRunNote, "seen")
	grace.do(state.RunAnnotate{})
	if len(grace.s.Errors) != 0 || grace.s.Run.Detail.Run.Note.V != "seen" {
		t.Errorf("saving the run after the tag went: errors %v, run %+v", grace.s.Errors, grace.s.Run.Detail.Run)
	}
}
