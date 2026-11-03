package state_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
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
	if got, _ := state.Update(s, state.TagsFetched{Err: errors.New("connection refused")}); got.Notice != (state.Notice{Text: "loading tags: connection refused", Failed: true}) {
		t.Errorf("a failed load says %+v", got.Notice)
	}
}

func TestChangesRefetchWhatTheyTouch(t *testing.T) {
	s := member()
	s.Budgets.Selected = 2
	s.Run.ID = 5
	runs := state.RunsFetch{Limit: state.RunsLimit}
	for handler, want := range map[string][]state.Command{
		"Uploads.Create":    {runs, state.BudgetsFetch{}, state.BudgetFetch{ID: 2}},
		"Runs.Annotate":     {runs, state.RunFetch{ID: 5}},
		"db.TagDelete":      {state.TagsFetch{}, runs},
		"Budgets.Reset":     {state.BudgetsFetch{}, state.BudgetFetch{ID: 2}},
		"Widgets.Create":    {state.BoardsFetch{}},
		"Session.SignIn":    nil,
		"Thesis.Record":     nil,
		"Dashboards.Create": {state.BoardsFetch{}},
	} {
		got, cmds := state.Update(s, state.Changed{Handler: handler, Path: "/p"})
		if !reflect.DeepEqual(cmds, want) || got.Live != handler+" /p" {
			t.Errorf("%s asked %v, want %v (live %q)", handler, cmds, want, got.Live)
		}
	}
	if _, cmds := state.Update(state.State{}, state.Changed{Handler: "Uploads.Create"}); len(cmds) != 0 {
		t.Errorf("a change reached a client signed out: %v", cmds)
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
