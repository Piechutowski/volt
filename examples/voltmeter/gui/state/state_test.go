package state_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

func member() state.State {
	s, _ := state.Update(state.State{}, state.Started{})
	s, _ = state.Update(s, state.SignedIn{User: db.User{ID: 1, Name: "Ada", Email: "ada@voltmeter.test"}})
	return s
}

func TestUpdateLeavesItsInputAlone(t *testing.T) {
	s := member()
	s, _ = state.Update(s, state.FieldEdit{Field: state.FieldTagSlug, Value: "nightly"})
	before := s.Fields[state.FieldTagSlug]
	next, _ := state.Update(s, state.FieldEdit{Field: state.FieldTagSlug, Value: "weekly"})
	if s.Fields[state.FieldTagSlug] != before || next.Fields[state.FieldTagSlug] != "weekly" {
		t.Errorf("an edit changed the state it was given: %q, then %q", s.Fields[state.FieldTagSlug], next.Fields[state.FieldTagSlug])
	}

	s.Runs.Page = wire.RunPage{Next: 30}
	s.Runs.Back = make([]int, 1, 8)
	a, _ := state.Update(s, state.RunsNext{})
	s.Runs.Page.Next = 40
	b, _ := state.Update(s, state.RunsNext{})
	if !slices.Equal(a.Runs.Back, []int{0, 0}) || a.Runs.After != 30 || b.Runs.After != 40 {
		t.Errorf("two pages on from one state share their history: %+v, %+v", a.Runs, b.Runs)
	}
	c, _ := state.Update(a, state.RunsPrev{})
	if c.Runs.After != 0 || len(c.Runs.Back) != 1 || len(a.Runs.Back) != 2 {
		t.Errorf("going back changed the page it left: %+v, %+v", a.Runs, c.Runs)
	}
}

func TestStaleAnswersAreDropped(t *testing.T) {
	s := member()
	s, _ = state.Update(s, state.RunsFilter{Tag: "nightly"})
	old := state.RunsFetched{Query: state.RunsFetch{Limit: state.RunsLimit}, Page: wire.RunPage{Total: 99}}
	if got, _ := state.Update(s, old); got.Runs.Loaded || got.Runs.Page.Total == 99 {
		t.Errorf("the unfiltered page answered after the filter was applied: %+v", got.Runs)
	}
	fresh := state.RunsFetched{Query: state.RunsFetch{Tag: "nightly", Limit: state.RunsLimit}, Page: wire.RunPage{Total: 1}}
	if got, _ := state.Update(s, fresh); !got.Runs.Loaded || got.Runs.Page.Total != 1 {
		t.Errorf("the filtered page = %+v", got.Runs)
	}

	s, cmds := state.Update(s, state.RunOpen{ID: 7})
	if !slices.Contains(cmds, state.Command(state.RunFetch{ID: 7})) || s.Screen != state.ScreenRun {
		t.Errorf("opening run 7 asked %v", cmds)
	}
	if got, _ := state.Update(s, state.RunFetched{ID: 6, Detail: wire.RunDetail{Run: db.Run{ID: 6}}}); got.Run.Loaded {
		t.Errorf("run 6 answered for run 7: %+v", got.Run)
	}
	s, _ = state.Update(s, state.ThesisPick{Fixture: "examples/voltmeter"})
	if got, _ := state.Update(s, state.ThesisFetched{Fixture: "itest/blog", Rows: []db.Thesis{{Sha: "aa"}}}); got.Thesis.Loaded {
		t.Errorf("another fixture's counts landed: %+v", got.Thesis)
	}
}

func TestLocalRefusalsAskNothing(t *testing.T) {
	s := member()
	s.Budgets.List = []db.Budget{{ID: 3, Phase: db.PhaseVet, Tables: 4, MaxMs: 2}}
	s, _ = state.Update(s, state.BudgetPick{ID: 3})
	if s.Fields[state.FieldCeiling] != "2" {
		t.Errorf("the ceiling field holds %q", s.Fields[state.FieldCeiling])
	}
	for _, c := range []struct {
		name  string
		edits map[state.Field]string
		in    state.Intent
		field state.Field
	}{
		{"a ceiling that is no number", map[state.Field]string{state.FieldCeiling: "fast"}, state.BudgetCeilingSave{}, state.FieldCeiling},
		{"a budget size that is no number", map[state.Field]string{state.FieldBudgetTables: "x", state.FieldBudgetMax: "3"}, state.BudgetAdd{}, state.FieldBudgetTables},
		{"a rename without a slug", map[state.Field]string{state.FieldTagSlug: " "}, state.TagRename{}, state.FieldTagSlug},
		{"a delete without a slug", map[state.Field]string{state.FieldTagSlug: ""}, state.TagRemove{}, state.FieldTagSlug},
	} {
		next := s
		for f, v := range c.edits {
			next, _ = state.Update(next, state.FieldEdit{Field: f, Value: v})
		}
		next, cmds := state.Update(next, c.in)
		if len(cmds) != 0 || next.Errors[c.field] == "" {
			t.Errorf("%s asked %v with errors %v", c.name, cmds, next.Errors)
		}
	}
	out, cmds := state.Update(state.State{}, state.SignInSubmit{})
	if len(cmds) != 0 || out.Errors[state.FieldEmail] == "" {
		t.Errorf("an empty sign-in asked %v", cmds)
	}
	if _, cmds := state.Update(out, state.ScreenShow{Screen: state.ScreenBudgets}); !reflect.DeepEqual(cmds, []state.Command{state.UsersFetch{}}) {
		t.Errorf("a screen asked while signed out = %v", cmds)
	}
}

func TestProblemDetailsLandUnderTheirFields(t *testing.T) {
	s := member()
	refused := &volt.ProblemError{Problem: volt.Problem{Status: 422, Message: "check the fields", Details: []volt.Detail{
		{Check: "PhaseKnown", Columns: []string{"phase"}, Message: `"link" is not a phase`},
		{Check: "BudgetSane", Columns: []string{"tables", "max_ms"}, Message: "tables must be at least 1"},
	}}}
	got, _ := state.Update(s, state.BudgetPosted{Err: refused})
	want := map[state.Field]string{
		state.FieldBudgetPhase:  `"link" is not a phase`,
		state.FieldBudgetTables: "tables must be at least 1",
		state.FieldBudgetMax:    "tables must be at least 1",
	}
	if !reflect.DeepEqual(got.Errors, want) || got.Notice != (state.Notice{Text: "422 check the fields", Failed: true}) {
		t.Errorf("errors %v, notice %+v", got.Errors, got.Notice)
	}
	fk := &volt.ProblemError{Problem: volt.Problem{Status: 422, Message: "Run: references a row that does not exist", Details: []volt.Detail{{Check: "foreign_key", Message: "references a row that does not exist"}}}}
	if got, _ := state.Update(s, state.RunPatched{Err: fk}); got.Errors[state.FieldRunTag] != "references a row that does not exist" {
		t.Errorf("a refused foreign key shows %v", got.Errors)
	}
	if got, _ := state.Update(s, state.TagsFetched{Err: errors.New("connection refused")}); got.Notice != (state.Notice{Text: "loading tags: connection refused", Failed: true, Of: "loading tags"}) {
		t.Errorf("a failed load says %+v", got.Notice)
	}
}

func TestChangesRefetchWhatTheyTouch(t *testing.T) {
	s := member()
	s.Budgets.Selected = 2
	s.Run.ID = 5
	runs := state.RunsFetch{Limit: state.RunsLimit}
	for handler, want := range map[string][]state.Command{
		"Uploads.Create":    {state.TagsFetch{}, runs, state.BudgetsFetch{}, state.BudgetFetch{ID: 2}},
		"Runs.Annotate":     {runs, state.RunFetch{ID: 5}},
		"db.TagDelete":      {state.TagsFetch{}, runs, state.RunFetch{ID: 5}},
		"Budgets.Reset":     {state.BudgetsFetch{}, state.BudgetFetch{ID: 2}},
		"Widgets.Create":    {state.BoardsFetch{}},
		"Session.SignIn":    nil,
		"Thesis.Record":     nil,
		"Dashboards.Create": {state.BoardsFetch{}},
	} {
		got, cmds := state.Update(s, state.Changed{Handler: handler, Path: "/p"})
		if !reflect.DeepEqual(cmds, want) || got.Live != state.ChangeText(state.Changed{Handler: handler, Path: "/p"}) {
			t.Errorf("%s asked %v, want %v (live %q)", handler, cmds, want, got.Live)
		}
		replayed, again := state.Update(s, state.Changed{Handler: handler, Path: "/p", Replayed: true})
		if !reflect.DeepEqual(again, want) || replayed.Live != "" {
			t.Errorf("%s replayed asked %v and set the live line %q; want the same refetch and no news", handler, again, replayed.Live)
		}
	}
	for c, want := range map[state.Changed]string{
		{Handler: "Samples.Create", Path: "/api/runs/2/samples"}:    "run 2 got route timings",
		{Handler: "Uploads.Create", Path: "/api/uploads"}:           "a run was uploaded",
		{Handler: "Runs.Delete", Path: "/api/runs/12"}:              "run 12 was deleted",
		{Handler: "Widgets.Create", Path: "/api/me/dashboards/1/w"}: "a dashboard changed",
		{Handler: "Things.Make", Path: "/things"}:                   "Things.Make /things",
	} {
		if got := state.ChangeText(c); got != want {
			t.Errorf("ChangeText(%s %s) = %q, want %q", c.Handler, c.Path, got, want)
		}
	}
	for n, want := range map[int64]string{0: "No budget was breached", 1: "Reset 1 budget to ok", 3: "Reset 3 budgets to ok"} {
		if got := state.ResetText(n); got != want {
			t.Errorf("ResetText(%d) = %q, want %q", n, got, want)
		}
	}
	at := time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)
	if got, _ := state.Update(s, state.Changed{Handler: "db.TagCreate", Path: "/p", At: at}); !got.LiveAt.Equal(at) {
		t.Errorf("the live line keeps the change's time %v, want %v", got.LiveAt, at)
	}
	if _, cmds := state.Update(state.State{Members: []db.User{{ID: 1}}}, state.Changed{Handler: "Uploads.Create"}); len(cmds) != 0 {
		t.Errorf("a change reached a client signed out: %v", cmds)
	}
	if _, cmds := state.Update(state.State{}, state.Changed{Handler: "Uploads.Create"}); !reflect.DeepEqual(cmds, []state.Command{state.UsersFetch{}}) {
		t.Errorf("a change reaching a signed-out client with no team asked %v, want the team again", cmds)
	}

	data, _ := json.Marshal(map[string]any{"method": "POST", "route": "POST /api/uploads", "handler": "Uploads.Create", "path": "/api/uploads", "status": 201})
	in, ok := state.ChangedOf(volt.Event{ID: 4, Name: "changed", Data: data})
	if !ok || in != (state.Changed{Method: "POST", Route: "POST /api/uploads", Handler: "Uploads.Create", Path: "/api/uploads", Status: 201}) {
		t.Errorf("ChangedOf = %+v, %v", in, ok)
	}
	if _, ok := state.ChangedOf(volt.Event{Name: "other", Data: data}); ok {
		t.Error("an event of another name became an intent")
	}
}

func TestBreachedBudgetIsPickedFirst(t *testing.T) {
	s := member()
	got, cmds := state.Update(s, state.BudgetsFetched{Budgets: []db.Budget{{ID: 1, State: db.HealthOK}, {ID: 2, State: db.HealthBreached, MaxMs: 2.5}}})
	if got.Budgets.Selected != 2 || got.Fields[state.FieldCeiling] != "2.5" || !reflect.DeepEqual(cmds, []state.Command{state.BudgetFetch{ID: 2}}) {
		t.Errorf("selected %d, ceiling %q, asked %v", got.Budgets.Selected, got.Fields[state.FieldCeiling], cmds)
	}
}

func TestChartFrame(t *testing.T) {
	f := state.Chart{Series: []state.Series{{Points: []state.Point{{X: 1, Y: 0.35}, {X: 2, Y: 0.38}}}}}.Frame()
	var labels []string
	for _, tk := range f.YTicks {
		labels = append(labels, tk.Label)
	}
	if f.YMax != 0.4 || !slices.Equal(labels, []string{"0.0", "0.1", "0.2", "0.3", "0.4"}) {
		t.Errorf("a frame to 0.38 = %v %v", f.YMax, labels)
	}
	f = state.Chart{Series: []state.Series{{Points: []state.Point{{X: 0, Y: 3050}}}}}.Frame()
	if f.YMax != 4000 || len(f.YTicks) != 5 || f.YTicks[4].Label != "4000" || f.XMin != -1 || f.XMax != 1 {
		t.Errorf("a frame of one point = %+v", f)
	}
	if x, y := f.At(state.Point{X: 0, Y: 2000}, 100, 50); x != 50 || y != 25 {
		t.Errorf("the middle lands at %v, %v", x, y)
	}
	if f := (state.Chart{}).Frame(); f.YMax != 1 || f.XMax != 1 || len(f.YTicks) != 2 {
		t.Errorf("an empty frame = %+v", f)
	}

	rows := make([]db.Thesis, 8)
	for i := range rows {
		rows[i] = db.Thesis{Sha: "abcdef" + string(rune('0'+i)), VoltLines: i}
	}
	var at []float64
	for _, tk := range state.ThesisChart(rows).XTicks {
		at = append(at, tk.Value)
	}
	if !slices.Equal(at, []float64{0, 2, 4, 7}) {
		t.Errorf("eight commits are labelled at %v", at)
	}
}

// TestEditedRunFieldsSurviveALiveRefetch proves a change event that
// refetches the open run while the member edits its note keeps the
// edit, so the save stores what the member typed, and a refetch after
// the save shows the server's values again.
func TestEditedRunFieldsSurviveALiveRefetch(t *testing.T) {
	s := member()
	s, _ = state.Update(s, state.RunOpen{ID: 5})
	run := db.Run{ID: 5, Note: rt.Some("old note")}
	s, _ = state.Update(s, state.RunFetched{ID: 5, Detail: wire.RunDetail{Run: run}})
	if s.Fields[state.FieldRunNote] != "old note" {
		t.Fatalf("the first load filled the note with %q", s.Fields[state.FieldRunNote])
	}
	s, _ = state.Update(s, state.FieldEdit{Field: state.FieldRunNote, Value: "old note xyz"})
	s, cmds := state.Update(s, state.Changed{Handler: "Samples.Create", Path: "/api/runs/5/samples"})
	if !slices.Contains(cmds, state.Command(state.RunFetch{ID: 5})) {
		t.Fatalf("a sample batch asked %v", cmds)
	}
	s, _ = state.Update(s, state.RunFetched{ID: 5, Detail: wire.RunDetail{Run: run}})
	if s.Fields[state.FieldRunNote] != "old note xyz" {
		t.Errorf("the refetch replaced the edited note with %q", s.Fields[state.FieldRunNote])
	}
	s, cmds = state.Update(s, state.RunAnnotate{})
	if !reflect.DeepEqual(cmds, []state.Command{state.RunPatch{ID: 5, Note: "old note xyz"}}) {
		t.Errorf("the save asked %v", cmds)
	}
	s, _ = state.Update(s, state.RunPatched{Run: db.Run{ID: 5, Note: rt.Some("old note xyz")}})
	s, _ = state.Update(s, state.RunFetched{ID: 5, Detail: wire.RunDetail{Run: db.Run{ID: 5, Note: rt.Some("from Grace")}}})
	if s.Fields[state.FieldRunNote] != "from Grace" {
		t.Errorf("after the save a refetch left the note at %q", s.Fields[state.FieldRunNote])
	}
}

// TestMeasurementsChangeRefetchesEveryTrendShown proves an upload or a
// deleted run refetches the trend of every spot the client holds, the
// dashboard cards' included, and the sizes it knows.
func TestMeasurementsChangeRefetchesEveryTrendShown(t *testing.T) {
	s := member()
	s, _ = state.Update(s, state.TrendFetched{Spot: state.Spot{Phase: db.PhaseVet, Tables: 40}, Points: []state.TrendPoint{{RunID: 1, WallMS: 2}}})
	s, _ = state.Update(s, state.TrendFetched{Spot: state.Spot{Phase: db.PhaseLoad, Tables: 10}, Points: []state.TrendPoint{{RunID: 1, WallMS: 1}}})
	s, _ = state.Update(s, state.SizesFetched{Sizes: []int{10, 40}})
	s, _ = state.Update(s, state.BoardsFetched{Boards: []db.DashboardWithWidgets{{ID: 1, Widgets: []db.Widget{{Phase: db.PhaseLoad, Tables: 10}}}}})
	want := []state.Command{state.SizesFetch{}, state.TrendFetch{Spot: state.Spot{Phase: db.PhaseLoad, Tables: 10}}, state.TrendFetch{Spot: state.Spot{Phase: db.PhaseVet, Tables: 40}}}
	_, cmds := state.Update(s, state.Changed{Handler: "Uploads.Create"})
	if !reflect.DeepEqual(cmds, append([]state.Command{state.TagsFetch{}, state.RunsFetch{Limit: state.RunsLimit}, state.BudgetsFetch{}}, append(want, state.BoardsFetch{})...)) {
		t.Errorf("an upload asked %v", cmds)
	}
	_, cmds = state.Update(s, state.Changed{Handler: "Runs.Delete"})
	if !reflect.DeepEqual(cmds, append([]state.Command{state.RunsFetch{Limit: state.RunsLimit}}, want...)) {
		t.Errorf("a deleted run asked %v", cmds)
	}
}

// TestTrendSizesComeFromTheServer proves the trend screen offers every
// size the server measured, not only the open run's, and moves off the
// default size when the server measured none of it.
func TestTrendSizesComeFromTheServer(t *testing.T) {
	s := member()
	s, cmds := state.Update(s, state.ScreenShow{Screen: state.ScreenTrends})
	if !reflect.DeepEqual(cmds, []state.Command{state.SizesFetch{}, state.TrendFetch{Spot: state.Spot{Phase: db.PhaseLoad, Tables: state.TablesDefault}}}) {
		t.Fatalf("the trend screen asked %v", cmds)
	}
	s, cmds = state.Update(s, state.SizesFetched{Sizes: []int{2, 4}})
	if s.Trend.Tables != 2 || !reflect.DeepEqual(cmds, []state.Command{state.TrendFetch{Spot: state.Spot{Phase: db.PhaseLoad, Tables: 2}}}) || !slices.Equal(s.Sizes(), []int{2, 4}) {
		t.Errorf("sizes 2 and 4 on a server without 10: trend %+v, asked %v, offered %v", s.Trend, cmds, s.Sizes())
	}
	s.Run.Detail.Phases = []wire.PhaseRow{{Tables: 160}}
	if !slices.Equal(s.Sizes(), []int{2, 4, 160}) {
		t.Errorf("with a run of 160 open, offered %v", s.Sizes())
	}
}

// TestNoticesAndDeletedRuns proves a failed load's notice clears when
// the same load succeeds or the member moves to another screen, and a
// run deleted while open closes rather than staying on screen.
func TestNoticesAndDeletedRuns(t *testing.T) {
	s := member()
	s, _ = state.Update(s, state.TagsFetched{Err: errors.New("connection refused")})
	if got, _ := state.Update(s, state.BudgetsFetched{}); !got.Notice.Failed {
		t.Errorf("another load's success cleared %+v", s.Notice)
	}
	if got, _ := state.Update(s, state.TagsFetched{}); got.Notice != (state.Notice{}) {
		t.Errorf("the tags loading again left %+v", got.Notice)
	}
	if got, _ := state.Update(s, state.ScreenShow{Screen: state.ScreenThesis}); got.Notice != (state.Notice{}) {
		t.Errorf("a new screen kept %+v", got.Notice)
	}

	s, _ = state.Update(s, state.RunOpen{ID: 5})
	s, _ = state.Update(s, state.RunFetched{ID: 5, Detail: wire.RunDetail{Run: db.Run{ID: 5, Note: rt.Some("n")}}})
	gone := &volt.ProblemError{Problem: volt.Problem{Status: 404, Message: "not found"}}
	s, _ = state.Update(s, state.RunFetched{ID: 5, Err: gone})
	if s.Run.ID != 0 || s.Run.Loaded || s.Fields[state.FieldRunNote] != "" || s.Notice.Failed {
		t.Errorf("a run deleted while open stays: %+v, note %q, notice %+v", s.Run, s.Fields[state.FieldRunNote], s.Notice)
	}
	if _, cmds := state.Update(s, state.RunAnnotate{}); len(cmds) != 0 {
		t.Errorf("annotating the deleted run asked %v", cmds)
	}
}

// TestAnswersForAnEarlierMemberAreDropped proves an answer to a command
// issued before a sign-out lands in no later member's state.
func TestAnswersForAnEarlierMemberAreDropped(t *testing.T) {
	s := member()
	s, cmds := state.Update(s, state.ScreenShow{Screen: state.ScreenDashboards})
	if !slices.Contains(cmds, state.Command(state.BoardsFetch{})) {
		t.Fatalf("dashboards asked %v", cmds)
	}
	ada := s.Session
	s, _ = state.Update(s, state.SignOut{})
	s, _ = state.Update(s, state.Answer{Session: s.Session, Result: state.SignedIn{User: db.User{ID: 2, Name: "Grace"}}})
	late := state.BoardsFetched{Boards: []db.DashboardWithWidgets{{ID: 9, OwnerID: 1, Name: "Ada's board"}}}
	if got, _ := state.Update(s, state.Answer{Session: ada, Result: late}); len(got.Boards.List) != 0 || got.User.ID != 2 {
		t.Errorf("Grace sees %+v", got.Boards.List)
	}
	if got, _ := state.Update(s, state.Answer{Session: s.Session, Result: late}); len(got.Boards.List) != 1 {
		t.Errorf("an answer of the current session was dropped: %+v", got.Boards)
	}
}

// TestSignedOutFailureIsShownAndRetried proves a client that could not
// load the team says so and asks again when the member retries, that
// the notice clears once the team loads, and that a sign-in failing
// for want of a server is a notice, not an error of the email field.
func TestSignedOutFailureIsShownAndRetried(t *testing.T) {
	s, _ := state.Update(state.State{}, state.Started{})
	s, _ = state.Update(s, state.UsersFetched{Err: errors.New("connection refused")})
	if s.Notice != (state.Notice{Text: "loading the team: connection refused", Failed: true, Of: "loading the team"}) {
		t.Errorf("a failed team load says %+v", s.Notice)
	}
	s, cmds := state.Update(s, state.MembersReload{})
	if !reflect.DeepEqual(cmds, []state.Command{state.UsersFetch{}}) {
		t.Errorf("retrying asked %v", cmds)
	}
	s, _ = state.Update(s, state.UsersFetched{Users: []db.User{{ID: 1, Name: "Ada"}}})
	if s.Notice != (state.Notice{}) || len(s.Members) != 1 {
		t.Errorf("after the team loads: notice %+v, members %v", s.Notice, s.Members)
	}
	down, _ := state.Update(s, state.SignedIn{Err: errors.New("dial tcp: connection refused")})
	if down.Errors[state.FieldEmail] != "" || down.Notice != (state.Notice{Text: "signing in: dial tcp: connection refused", Failed: true, Of: "signing in"}) {
		t.Errorf("a sign-in that reached no server: errors %v, notice %+v; the address is not at fault", down.Errors, down.Notice)
	}
	down.Fields = map[state.Field]string{state.FieldEmail: "ada@voltmeter.test"}
	if again, cmds := state.Update(down, state.SignInSubmit{}); again.Notice != (state.Notice{}) || !reflect.DeepEqual(cmds, []state.Command{state.SignInPost{Email: "ada@voltmeter.test"}}) {
		t.Errorf("signing in again kept the notice %+v and asked %v", again.Notice, cmds)
	}
}

// TestUnknownEmailIsTheFieldsAlone proves a refused sign-in puts the
// server's message under the email field once, without its status and
// without a banner, and that submitting again replaces it rather than
// leaving a stale one beside the new finding.
func TestUnknownEmailIsTheFieldsAlone(t *testing.T) {
	s, _ := state.Update(state.State{}, state.Started{})
	s.Fields = map[state.Field]string{state.FieldEmail: "nobody@voltmeter.test"}
	s, _ = state.Update(s, state.SignInSubmit{})
	refused := &volt.ProblemError{Problem: volt.Problem{Status: 401, Message: "no member signs in as nobody@voltmeter.test"}}
	s, _ = state.Update(s, state.SignedIn{Err: refused})
	if s.Errors[state.FieldEmail] != "no member signs in as nobody@voltmeter.test" || s.Notice != (state.Notice{}) {
		t.Errorf("an unknown email: errors %v, notice %+v", s.Errors, s.Notice)
	}
	s.Fields = map[state.Field]string{state.FieldEmail: ""}
	s, _ = state.Update(s, state.SignInSubmit{})
	if s.Errors[state.FieldEmail] != "name a member by email" || s.Notice != (state.Notice{}) {
		t.Errorf("an empty email after it: errors %v, notice %+v", s.Errors, s.Notice)
	}
}

// TestCeilingFollowsAnotherMembersChange proves the ceiling field takes
// a refetched ceiling while the member has not typed in it, and keeps
// what the member typed otherwise, so saving never silently reverts
// another member's change.
func TestCeilingFollowsAnotherMembersChange(t *testing.T) {
	s := member()
	list := []db.Budget{{ID: 1, Phase: db.PhaseLoad, Tables: 2, MaxMs: 5}, {ID: 2, Phase: db.PhaseVet, Tables: 2, MaxMs: 50}}
	s, _ = state.Update(s, state.BudgetsFetched{Budgets: list})
	s, _ = state.Update(s, state.BudgetPick{ID: 2})
	if s.Fields[state.FieldCeiling] != "50" {
		t.Fatalf("picked ceiling %q", s.Fields[state.FieldCeiling])
	}
	changed := []db.Budget{list[0], {ID: 2, Phase: db.PhaseVet, Tables: 2, MaxMs: 20}}
	got, _ := state.Update(s, state.BudgetsFetched{Budgets: changed})
	if got.Fields[state.FieldCeiling] != "20" {
		t.Errorf("after another member set 20 the field reads %q", got.Fields[state.FieldCeiling])
	}
	typed, _ := state.Update(s, state.FieldEdit{Field: state.FieldCeiling, Value: "35"})
	typed, _ = state.Update(typed, state.BudgetsFetched{Budgets: changed})
	if typed.Fields[state.FieldCeiling] != "35" {
		t.Errorf("a refetch overwrote the typed ceiling: %q", typed.Fields[state.FieldCeiling])
	}
	saved, _ := state.Update(typed, state.BudgetPatched{Budget: db.Budget{ID: 2, MaxMs: 35}})
	saved, _ = state.Update(saved, state.BudgetsFetched{Budgets: []db.Budget{list[0], {ID: 2, MaxMs: 40}}})
	if saved.Fields[state.FieldCeiling] != "40" {
		t.Errorf("after saving, a later change by another member reads %q, want 40", saved.Fields[state.FieldCeiling])
	}
}

// TestBudgetStateTextAgreesWithItsBreaches proves the budget's state
// line never says no upload went over beside a listed breach without
// saying the breach came before the reset, and that a breached budget
// names the newest breach.
func TestBudgetStateTextAgreesWithItsBreaches(t *testing.T) {
	ok := db.Budget{ID: 1, Phase: db.PhaseGenerate, Tables: 160, MaxMs: 20, State: db.HealthOK}
	breaches := []db.Breach{{ID: 2, RunID: 4, WallMs: 25.4}, {ID: 1, RunID: 3, WallMs: 33.7}}
	if text, danger := state.BudgetStateText(ok, nil); danger || !strings.Contains(text, "since this budget was set") {
		t.Errorf("a fresh ok budget says %q (warning %v)", text, danger)
	}
	if text, danger := state.BudgetStateText(ok, breaches); danger || !strings.Contains(text, "came before the reset") {
		t.Errorf("an ok budget with breaches says %q (warning %v)", text, danger)
	}
	broken := ok
	broken.State = db.HealthBreached
	if text, danger := state.BudgetStateText(broken, breaches); !danger || !strings.Contains(text, "run 4 took 25.4 ms") {
		t.Errorf("a breached budget says %q (warning %v), want the newest breach, run 4", text, danger)
	}
}

// TestRunJudgedTextNamesBaselineAndBreaches proves the run screen says
// which baseline a run was collected against and which budgets it went
// over, so a breaching run never looks like any other.
func TestRunJudgedTextNamesBaselineAndBreaches(t *testing.T) {
	plain := wire.RunDetail{Run: db.Run{ID: 2}}
	if text, over := state.RunJudgedText(plain); over || text != "No baseline. Within every budget set when it was uploaded." {
		t.Errorf("a plain run says %q (over %v)", text, over)
	}
	broke := wire.RunDetail{Run: db.Run{ID: 3, BaselineID: rt.Some(1)}, Breaches: []db.Breach{{BudgetID: 1, RunID: 3, WallMs: 33.7}, {BudgetID: 9, RunID: 3, WallMs: 5}},
		Over: []db.Budget{{ID: 1, Phase: db.PhaseGenerate, Tables: 160}}}
	want := "Baseline: run 1. Over budget: generate at 160 tables, took 33.7 ms; budget 9, took 5.0 ms."
	if text, over := state.RunJudgedText(broke); !over || text != want {
		t.Errorf("a breaching run says %q (over %v), want %q", text, over, want)
	}
}
