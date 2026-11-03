// Package state is the Voltmeter desktop client without its window: a
// State value, the Intent values that change it, and the Command values
// that describe what the client asks the server, joined by the pure
// function Update (spec §values-and-effects). The widgets render a State
// and emit intents, and a runner performs the commands and answers each
// with a result intent, so the whole client is driven and tested
// without a screen.
package state

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

// RunsLimit is the size of one page of the run list.
const RunsLimit = 15

// TablesDefault is the size the trend screen shows before any run
// names the sizes it measured.
const TablesDefault = 10

// Screen is one screen of the client.
type Screen int

const (
	ScreenSignIn Screen = iota
	ScreenRuns
	ScreenRun
	ScreenTrends
	ScreenThesis
	ScreenBudgets
	ScreenDashboards
)

// Screens are the screens a signed-in member moves between, in the
// order the sidebar lists them.
var Screens = []Screen{ScreenRuns, ScreenRun, ScreenTrends, ScreenThesis, ScreenBudgets, ScreenDashboards}

// String is the screen's title.
func (s Screen) String() string {
	switch s {
	case ScreenSignIn:
		return "Sign in"
	case ScreenRuns:
		return "Runs"
	case ScreenRun:
		return "Run"
	case ScreenTrends:
		return "Trends"
	case ScreenThesis:
		return "Thesis"
	case ScreenBudgets:
		return "Budgets"
	case ScreenDashboards:
		return "Dashboards"
	}
	return "Screen " + strconv.Itoa(int(s))
}

// Field names one text input of the client; State keeps what each
// holds and the error shown under it.
type Field string

const (
	FieldEmail        Field = "email"
	FieldTagSlug      Field = "tag_slug"
	FieldTagName      Field = "tag_name"
	FieldRunTag       Field = "run_tag"
	FieldRunNote      Field = "run_note"
	FieldCeiling      Field = "ceiling"
	FieldBudgetPhase  Field = "budget_phase"
	FieldBudgetTables Field = "budget_tables"
	FieldBudgetMax    Field = "budget_max"
	FieldBoardName    Field = "board_name"
	FieldWidgetPhase  Field = "widget_phase"
	FieldWidgetTables Field = "widget_tables"
)

// Spot is one phase at one size, what a trend and a dashboard widget
// follow across runs.
type Spot struct {
	Phase  db.Phase
	Tables int
}

// TrendPoint is one run's wall time at a spot.
type TrendPoint struct {
	RunID  int
	WallMS units.Millis
}

// Notice is the last outcome the client reports in its header.
type Notice struct {
	Text   string
	Failed bool
}

// RunList is the keyset-paged run list: After is the cursor the shown
// page was asked with and Back the cursors of the pages before it.
type RunList struct {
	Tag    string
	After  int
	Back   []int
	Page   app.RunPage
	Loaded bool
}

// RunView is the run the run screen shows and where its CSV went.
type RunView struct {
	ID     int
	Detail app.RunDetail
	Loaded bool
	Export string
}

// ThesisView is the line counts of one fixture, oldest commit first.
type ThesisView struct {
	Fixture string
	Rows    []db.Thesis
	Loaded  bool
}

// BudgetView is every budget and the one selected, with its breaches.
type BudgetView struct {
	List     []db.Budget
	Selected int
	Detail   app.BudgetDetail
	Loaded   bool
}

// BoardView is the member's dashboards and the one selected.
type BoardView struct {
	List     []db.DashboardWithWidgets
	Selected int
	Loaded   bool
}

// State is everything the client shows. Update returns a new State and
// never changes the slices or maps of the one it is given.
type State struct {
	Screen  Screen
	Members []db.User
	User    db.User
	Tags    []db.Tag
	Runs    RunList
	Run     RunView
	Trend   Spot
	Trends  map[Spot][]TrendPoint
	Thesis  ThesisView
	Budgets BudgetView
	Boards  BoardView
	Fields  map[Field]string
	Errors  map[Field]string
	Notice  Notice
	Live    string
}

// Intent is something that happened: a member acted, a command
// answered, or the server published a change.
type Intent interface{ intent() }

// Command is a request the client makes of the server, performed by
// the runner, which answers it with one result intent.
type Command interface{ command() }

// Started opens the client on the sign-in screen.
type Started struct{}

// FieldEdit is new text in a field.
type FieldEdit struct {
	Field Field
	Value string
}

// MemberPick signs in as the member with the email.
type MemberPick struct{ Email string }

// SignInSubmit signs in as the email the email field holds.
type SignInSubmit struct{}

// SignOut forgets the member.
type SignOut struct{}

// ScreenShow moves to a screen and loads what it shows.
type ScreenShow struct{ Screen Screen }

// RunsFilter shows the runs carrying a tag, every run for "".
type RunsFilter struct{ Tag string }

// RunsNext shows the next page of runs.
type RunsNext struct{}

// RunsPrev shows the page before.
type RunsPrev struct{}

// RunOpen shows one run.
type RunOpen struct{ ID int }

// RunAnnotate stores the tag and note fields on the open run.
type RunAnnotate struct{}

// RunExport saves the open run's CSV.
type RunExport struct{}

// TagAdd adds the tag the slug and name fields hold.
type TagAdd struct{}

// TagRename gives the tag the slug field names the name field's name.
type TagRename struct{}

// TagRemove deletes the tag the slug field names.
type TagRemove struct{}

// TrendPick follows a phase at a size across runs.
type TrendPick struct{ Spot Spot }

// ThesisPick shows a fixture's line counts.
type ThesisPick struct{ Fixture string }

// BudgetPick selects a budget and loads its breaches.
type BudgetPick struct{ ID int }

// BudgetCeilingSave sets the selected budget's ceiling to the ceiling
// field.
type BudgetCeilingSave struct{}

// BudgetAdd adds a budget from the budget fields.
type BudgetAdd struct{}

// BudgetsReset returns every breached budget to ok.
type BudgetsReset struct{}

// BoardPick selects a dashboard.
type BoardPick struct{ ID int }

// BoardAdd adds a dashboard named by the board name field.
type BoardAdd struct{}

// WidgetAdd adds a widget from the widget fields to the selected
// dashboard.
type WidgetAdd struct{}

// WidgetRemove takes a widget off the selected dashboard.
type WidgetRemove struct{ ID int }

// Changed is the server's change event (spec §event-routes): which
// route wrote, so the client refetches what it shows.
type Changed struct {
	Method  string `json:"method"`
	Route   string `json:"route"`
	Handler string `json:"handler"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
}

// UsersFetch asks for the team.
type UsersFetch struct{}

// SignInPost signs in by email.
type SignInPost struct{ Email string }

// CookiesDrop forgets the sign-in cookie.
type CookiesDrop struct{}

// RunsFetch asks for one page of runs.
type RunsFetch struct {
	Tag   string
	After int
	Limit int
}

// RunFetch asks for everything one run measured.
type RunFetch struct{ ID int }

// RunPatch sets a run's tag and note; an empty tag or note clears it.
type RunPatch struct {
	ID   int
	Tag  string
	Note string
}

// RunCSVSave saves a run's CSV export to a file.
type RunCSVSave struct{ ID int }

// TagsFetch asks for every tag.
type TagsFetch struct{}

// TagPost adds a tag.
type TagPost struct{ Slug, Name string }

// TagPatch renames a tag.
type TagPatch struct{ Slug, Name string }

// TagDelete deletes a tag.
type TagDelete struct{ Slug string }

// TrendFetch asks for a spot's wall time across runs.
type TrendFetch struct{ Spot Spot }

// ThesisFetch asks for a fixture's line counts.
type ThesisFetch struct{ Fixture string }

// BudgetsFetch asks for every budget.
type BudgetsFetch struct{}

// BudgetFetch asks for one budget and its breaches.
type BudgetFetch struct{ ID int }

// BudgetPatch sets a budget's ceiling.
type BudgetPatch struct {
	ID    int
	MaxMS float64
}

// BudgetPost adds a budget.
type BudgetPost struct {
	Phase  db.Phase
	Tables int
	MaxMS  float64
}

// BudgetsResetPost returns every breached budget to ok.
type BudgetsResetPost struct{}

// BoardsFetch asks for the member's dashboards with their widgets.
type BoardsFetch struct{}

// BoardPost adds a dashboard.
type BoardPost struct{ Name string }

// WidgetPost adds a widget to a dashboard.
type WidgetPost struct {
	Board    int
	Position int
	Phase    db.Phase
	Tables   int
}

// WidgetDelete takes a widget off a dashboard.
type WidgetDelete struct{ Board, ID int }

// UsersFetched answers UsersFetch.
type UsersFetched struct {
	Users []db.User
	Err   error
}

// SignedIn answers SignInPost.
type SignedIn struct {
	User db.User
	Err  error
}

// RunsFetched answers RunsFetch, naming the page it asked for.
type RunsFetched struct {
	Query RunsFetch
	Page  app.RunPage
	Err   error
}

// RunFetched answers RunFetch.
type RunFetched struct {
	ID     int
	Detail app.RunDetail
	Err    error
}

// RunPatched answers RunPatch.
type RunPatched struct {
	Run db.Run
	Err error
}

// RunCSVSaved answers RunCSVSave with the file written.
type RunCSVSaved struct {
	ID   int
	Path string
	Size int
	Err  error
}

// TagsFetched answers TagsFetch.
type TagsFetched struct {
	Tags []db.Tag
	Err  error
}

// TagSaved answers TagPost and TagPatch.
type TagSaved struct {
	Tag db.Tag
	Err error
}

// TagDeleted answers TagDelete.
type TagDeleted struct {
	Slug string
	Err  error
}

// TrendFetched answers TrendFetch.
type TrendFetched struct {
	Spot   Spot
	Points []TrendPoint
	Err    error
}

// ThesisFetched answers ThesisFetch.
type ThesisFetched struct {
	Fixture string
	Rows    []db.Thesis
	Err     error
}

// BudgetsFetched answers BudgetsFetch.
type BudgetsFetched struct {
	Budgets []db.Budget
	Err     error
}

// BudgetFetched answers BudgetFetch.
type BudgetFetched struct {
	ID     int
	Detail app.BudgetDetail
	Err    error
}

// BudgetPatched answers BudgetPatch.
type BudgetPatched struct {
	Budget db.Budget
	Err    error
}

// BudgetPosted answers BudgetPost.
type BudgetPosted struct {
	Budget db.Budget
	Err    error
}

// BudgetsResetPosted answers BudgetsResetPost.
type BudgetsResetPosted struct {
	Reset app.Reset
	Err   error
}

// BoardsFetched answers BoardsFetch.
type BoardsFetched struct {
	Boards []db.DashboardWithWidgets
	Err    error
}

// BoardPosted answers BoardPost.
type BoardPosted struct {
	Board db.Dashboard
	Err   error
}

// WidgetPosted answers WidgetPost.
type WidgetPosted struct {
	Widget db.Widget
	Err    error
}

// WidgetDeleted answers WidgetDelete.
type WidgetDeleted struct {
	ID  int
	Err error
}

func (Started) intent()            {}
func (FieldEdit) intent()          {}
func (MemberPick) intent()         {}
func (SignInSubmit) intent()       {}
func (SignOut) intent()            {}
func (ScreenShow) intent()         {}
func (RunsFilter) intent()         {}
func (RunsNext) intent()           {}
func (RunsPrev) intent()           {}
func (RunOpen) intent()            {}
func (RunAnnotate) intent()        {}
func (RunExport) intent()          {}
func (TagAdd) intent()             {}
func (TagRename) intent()          {}
func (TagRemove) intent()          {}
func (TrendPick) intent()          {}
func (ThesisPick) intent()         {}
func (BudgetPick) intent()         {}
func (BudgetCeilingSave) intent()  {}
func (BudgetAdd) intent()          {}
func (BudgetsReset) intent()       {}
func (BoardPick) intent()          {}
func (BoardAdd) intent()           {}
func (WidgetAdd) intent()          {}
func (WidgetRemove) intent()       {}
func (Changed) intent()            {}
func (UsersFetched) intent()       {}
func (SignedIn) intent()           {}
func (RunsFetched) intent()        {}
func (RunFetched) intent()         {}
func (RunPatched) intent()         {}
func (RunCSVSaved) intent()        {}
func (TagsFetched) intent()        {}
func (TagSaved) intent()           {}
func (TagDeleted) intent()         {}
func (TrendFetched) intent()       {}
func (ThesisFetched) intent()      {}
func (BudgetsFetched) intent()     {}
func (BudgetFetched) intent()      {}
func (BudgetPatched) intent()      {}
func (BudgetPosted) intent()       {}
func (BudgetsResetPosted) intent() {}
func (BoardsFetched) intent()      {}
func (BoardPosted) intent()        {}
func (WidgetPosted) intent()       {}
func (WidgetDeleted) intent()      {}

func (UsersFetch) command()       {}
func (SignInPost) command()       {}
func (CookiesDrop) command()      {}
func (RunsFetch) command()        {}
func (RunFetch) command()         {}
func (RunPatch) command()         {}
func (RunCSVSave) command()       {}
func (TagsFetch) command()        {}
func (TagPost) command()          {}
func (TagPatch) command()         {}
func (TagDelete) command()        {}
func (TrendFetch) command()       {}
func (ThesisFetch) command()      {}
func (BudgetsFetch) command()     {}
func (BudgetFetch) command()      {}
func (BudgetPatch) command()      {}
func (BudgetPost) command()       {}
func (BudgetsResetPost) command() {}
func (BoardsFetch) command()      {}
func (BoardPost) command()        {}
func (WidgetPost) command()       {}
func (WidgetDelete) command()     {}

// Fixtures are the fixtures the thesis screen offers, the collector's
// defaults.
var Fixtures = []string{"itest/blog", "examples/voltmeter"}

type form struct {
	columns map[string]Field
	checks  map[string]Field
}

var (
	tagForm     = form{columns: map[string]Field{"slug": FieldTagSlug, "name": FieldTagName}}
	runForm     = form{columns: map[string]Field{"tag": FieldRunTag, "note": FieldRunNote}, checks: map[string]Field{"foreign_key": FieldRunTag}}
	ceilingForm = form{columns: map[string]Field{"max_ms": FieldCeiling}}
	budgetForm  = form{columns: map[string]Field{"phase": FieldBudgetPhase, "tables": FieldBudgetTables, "max_ms": FieldBudgetMax}}
	boardForm   = form{columns: map[string]Field{"name": FieldBoardName}}
	widgetForm  = form{columns: map[string]Field{"phase": FieldWidgetPhase, "tables": FieldWidgetTables, "position": FieldWidgetTables}}
	emailForm   = form{columns: map[string]Field{"email": FieldEmail}}
)

// Update is the client's one transition: the state after in and the
// commands it asks the runner to perform.
func Update(s State, in Intent) (State, []Command) {
	switch in := in.(type) {
	case Started:
		return State{Screen: ScreenSignIn, Members: s.Members}, []Command{UsersFetch{}}
	case FieldEdit:
		s.Fields = with(s.Fields, in.Field, in.Value)
		return s, nil
	case MemberPick:
		s.Fields = with(s.Fields, FieldEmail, in.Email)
		return Update(s, SignInSubmit{})
	case SignInSubmit:
		email := strings.TrimSpace(s.Fields[FieldEmail])
		s.Errors = without(s.Errors, FieldEmail)
		if email == "" {
			s.Errors = with(s.Errors, FieldEmail, "name a member by email")
			return s, nil
		}
		return s, []Command{SignInPost{Email: email}}
	case SignOut:
		return State{Screen: ScreenSignIn, Members: s.Members}, []Command{CookiesDrop{}}
	case ScreenShow:
		return screenShow(s, in.Screen)
	case Changed:
		return changed(s, in)
	}
	if s.User.ID == 0 {
		return signedOut(s, in)
	}
	return member(s, in)
}

func signedOut(s State, in Intent) (State, []Command) {
	switch in := in.(type) {
	case UsersFetched:
		if in.Err != nil {
			return failed(s, "loading the team", in.Err), nil
		}
		s.Members = in.Users
		return s, nil
	case SignedIn:
		if in.Err != nil {
			s, placed := formFailed(s, in.Err, emailForm)
			if !placed {
				s.Errors = with(s.Errors, FieldEmail, problemText(in.Err))
			}
			return s, nil
		}
		s.User = in.User
		s.Notice = Notice{Text: "Signed in as " + in.User.Name}
		return screenShow(s, ScreenRuns)
	}
	return s, nil
}

func member(s State, in Intent) (State, []Command) {
	switch in := in.(type) {
	case SignedIn:
		if in.Err == nil {
			s.User = in.User
		}
		return s, nil
	case UsersFetched:
		if in.Err == nil {
			s.Members = in.Users
		}
		return s, nil
	case RunsFilter:
		s.Runs = RunList{Tag: in.Tag, Page: s.Runs.Page}
		return s, []Command{runsFetch(s)}
	case RunsNext:
		if s.Runs.Page.Next == 0 {
			return s, nil
		}
		s.Runs.Back = append(slices.Clip(s.Runs.Back), s.Runs.After)
		s.Runs.After = s.Runs.Page.Next
		return s, []Command{runsFetch(s)}
	case RunsPrev:
		if len(s.Runs.Back) == 0 {
			return s, nil
		}
		s.Runs.After = s.Runs.Back[len(s.Runs.Back)-1]
		s.Runs.Back = slices.Clip(s.Runs.Back[:len(s.Runs.Back)-1])
		return s, []Command{runsFetch(s)}
	case RunsFetched:
		if in.Query != runsFetch(s) {
			return s, nil
		}
		if in.Err != nil {
			return failed(s, "loading runs", in.Err), nil
		}
		s.Runs.Page, s.Runs.Loaded = in.Page, true
		return s, nil
	case RunOpen:
		s.Screen = ScreenRun
		if in.ID != s.Run.ID {
			s.Run = RunView{ID: in.ID}
		}
		return s, []Command{RunFetch{ID: in.ID}, TagsFetch{}}
	case RunFetched:
		if in.ID != s.Run.ID {
			return s, nil
		}
		if in.Err != nil {
			return failed(s, "loading run "+strconv.Itoa(in.ID), in.Err), nil
		}
		s.Run.Detail, s.Run.Loaded = in.Detail, true
		s.Fields = with(s.Fields, FieldRunTag, in.Detail.Run.Tag.V)
		s.Fields = with(s.Fields, FieldRunNote, in.Detail.Run.Note.V)
		if s.Trend.Tables == 0 && len(in.Detail.Phases) > 0 {
			s.Trend = Spot{Phase: db.PhaseLoad, Tables: in.Detail.Phases[0].Tables}
		}
		return s, nil
	case RunAnnotate:
		if s.Run.ID == 0 {
			return s, nil
		}
		s.Errors = without(s.Errors, FieldRunTag, FieldRunNote)
		return s, []Command{RunPatch{ID: s.Run.ID, Tag: strings.TrimSpace(s.Fields[FieldRunTag]), Note: s.Fields[FieldRunNote]}}
	case RunPatched:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, runForm)
			return s, nil
		}
		if in.Run.ID == s.Run.ID {
			s.Run.Detail.Run = in.Run
		}
		s.Notice = Notice{Text: "Saved run " + strconv.Itoa(in.Run.ID)}
		return s, []Command{runsFetch(s)}
	case RunExport:
		if s.Run.ID == 0 {
			return s, nil
		}
		return s, []Command{RunCSVSave{ID: s.Run.ID}}
	case RunCSVSaved:
		if in.Err != nil {
			return failed(s, "exporting run "+strconv.Itoa(in.ID), in.Err), nil
		}
		if in.ID == s.Run.ID {
			s.Run.Export = in.Path
		}
		s.Notice = Notice{Text: "Saved " + strconv.Itoa(in.Size) + " bytes to " + in.Path}
		return s, nil
	case TagsFetched:
		if in.Err != nil {
			return failed(s, "loading tags", in.Err), nil
		}
		s.Tags = in.Tags
		return s, nil
	case TagAdd:
		s.Errors = without(s.Errors, FieldTagSlug, FieldTagName)
		return s, []Command{TagPost{Slug: strings.TrimSpace(s.Fields[FieldTagSlug]), Name: strings.TrimSpace(s.Fields[FieldTagName])}}
	case TagRename:
		s.Errors = without(s.Errors, FieldTagSlug, FieldTagName)
		slug := strings.TrimSpace(s.Fields[FieldTagSlug])
		if slug == "" {
			s.Errors = with(s.Errors, FieldTagSlug, "name the tag to rename by its slug")
			return s, nil
		}
		return s, []Command{TagPatch{Slug: slug, Name: strings.TrimSpace(s.Fields[FieldTagName])}}
	case TagRemove:
		s.Errors = without(s.Errors, FieldTagSlug, FieldTagName)
		slug := strings.TrimSpace(s.Fields[FieldTagSlug])
		if slug == "" {
			s.Errors = with(s.Errors, FieldTagSlug, "name the tag to delete by its slug")
			return s, nil
		}
		return s, []Command{TagDelete{Slug: slug}}
	case TagSaved:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, tagForm)
			return s, nil
		}
		s.Fields = without(s.Fields, FieldTagSlug, FieldTagName)
		s.Notice = Notice{Text: "Saved tag " + in.Tag.Slug}
		return s, []Command{TagsFetch{}, runsFetch(s)}
	case TagDeleted:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, tagForm)
			return s, nil
		}
		s.Fields = without(s.Fields, FieldTagSlug, FieldTagName)
		s.Notice = Notice{Text: "Deleted tag " + in.Slug}
		if s.Runs.Tag == in.Slug {
			s.Runs = RunList{}
		}
		return s, []Command{TagsFetch{}, runsFetch(s)}
	case TrendPick:
		s.Trend = in.Spot
		return s, []Command{TrendFetch{Spot: in.Spot}}
	case TrendFetched:
		if in.Err != nil {
			return failed(s, "loading the "+string(in.Spot.Phase)+" trend", in.Err), nil
		}
		s.Trends = with(s.Trends, in.Spot, in.Points)
		return s, nil
	case ThesisPick:
		s.Thesis = ThesisView{Fixture: in.Fixture}
		return s, []Command{ThesisFetch{Fixture: in.Fixture}}
	case ThesisFetched:
		if in.Fixture != s.Thesis.Fixture {
			return s, nil
		}
		if in.Err != nil {
			return failed(s, "loading the thesis", in.Err), nil
		}
		s.Thesis.Rows, s.Thesis.Loaded = in.Rows, true
		return s, nil
	case BudgetsFetched:
		if in.Err != nil {
			return failed(s, "loading budgets", in.Err), nil
		}
		s.Budgets.List, s.Budgets.Loaded = in.Budgets, true
		if s.Budgets.Selected == 0 && len(in.Budgets) > 0 {
			first := in.Budgets[0].ID
			if i := slices.IndexFunc(in.Budgets, func(b db.Budget) bool { return b.State == db.HealthBreached }); i >= 0 {
				first = in.Budgets[i].ID
			}
			return Update(s, BudgetPick{ID: first})
		}
		return s, nil
	case BudgetPick:
		s.Budgets.Selected = in.ID
		s.Errors = without(s.Errors, FieldCeiling)
		if b, ok := s.budget(in.ID); ok {
			s.Fields = with(s.Fields, FieldCeiling, strconv.FormatFloat(b.MaxMs, 'f', -1, 64))
		}
		return s, []Command{BudgetFetch{ID: in.ID}}
	case BudgetFetched:
		if in.ID != s.Budgets.Selected {
			return s, nil
		}
		if in.Err != nil {
			return failed(s, "loading budget "+strconv.Itoa(in.ID), in.Err), nil
		}
		s.Budgets.Detail = in.Detail
		return s, nil
	case BudgetCeilingSave:
		s.Errors = without(s.Errors, FieldCeiling)
		if s.Budgets.Selected == 0 {
			return s, nil
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(s.Fields[FieldCeiling]), 64)
		if err != nil {
			s.Errors = with(s.Errors, FieldCeiling, "the ceiling is a number of milliseconds")
			return s, nil
		}
		return s, []Command{BudgetPatch{ID: s.Budgets.Selected, MaxMS: v}}
	case BudgetPatched:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, ceilingForm)
			return s, nil
		}
		s.Notice = Notice{Text: "Budget " + strconv.Itoa(in.Budget.ID) + " now allows " + units.Millis(in.Budget.MaxMs).String()}
		return s, []Command{BudgetsFetch{}, BudgetFetch{ID: in.Budget.ID}}
	case BudgetAdd:
		s.Errors = without(s.Errors, FieldBudgetPhase, FieldBudgetTables, FieldBudgetMax)
		tables, terr := strconv.Atoi(strings.TrimSpace(s.Fields[FieldBudgetTables]))
		if terr != nil {
			s.Errors = with(s.Errors, FieldBudgetTables, "the size is a whole number of tables")
		}
		maxMS, merr := strconv.ParseFloat(strings.TrimSpace(s.Fields[FieldBudgetMax]), 64)
		if merr != nil {
			s.Errors = with(s.Errors, FieldBudgetMax, "the ceiling is a number of milliseconds")
		}
		if terr != nil || merr != nil {
			return s, nil
		}
		return s, []Command{BudgetPost{Phase: db.Phase(s.Fields[FieldBudgetPhase]), Tables: tables, MaxMS: maxMS}}
	case BudgetPosted:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, budgetForm)
			return s, nil
		}
		s.Fields = without(s.Fields, FieldBudgetTables, FieldBudgetMax)
		s.Notice = Notice{Text: "Added budget " + strconv.Itoa(in.Budget.ID)}
		s.Budgets.Selected = in.Budget.ID
		s.Fields = with(s.Fields, FieldCeiling, strconv.FormatFloat(in.Budget.MaxMs, 'f', -1, 64))
		return s, []Command{BudgetsFetch{}, BudgetFetch{ID: in.Budget.ID}}
	case BudgetsReset:
		return s, []Command{BudgetsResetPost{}}
	case BudgetsResetPosted:
		if in.Err != nil {
			return failed(s, "resetting budgets", in.Err), nil
		}
		s.Notice = Notice{Text: "Reset " + strconv.FormatInt(in.Reset.Reset, 10) + " budgets to ok"}
		return s, budgetsFetch(s)
	case BoardsFetched:
		if in.Err != nil {
			return failed(s, "loading dashboards", in.Err), nil
		}
		s.Boards.List, s.Boards.Loaded = in.Boards, true
		if _, ok := s.board(s.Boards.Selected); !ok {
			s.Boards.Selected = 0
			if len(in.Boards) > 0 {
				s.Boards.Selected = in.Boards[0].ID
			}
		}
		var cmds []Command
		for _, b := range in.Boards {
			for _, w := range b.Widgets {
				spot := Spot{Phase: w.Phase, Tables: w.Tables}
				if _, ok := s.Trends[spot]; !ok && !slices.Contains(cmds, Command(TrendFetch{Spot: spot})) {
					cmds = append(cmds, TrendFetch{Spot: spot})
				}
			}
		}
		return s, cmds
	case BoardPick:
		s.Boards.Selected = in.ID
		return s, nil
	case BoardAdd:
		s.Errors = without(s.Errors, FieldBoardName)
		return s, []Command{BoardPost{Name: strings.TrimSpace(s.Fields[FieldBoardName])}}
	case BoardPosted:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, boardForm)
			return s, nil
		}
		s.Fields = without(s.Fields, FieldBoardName)
		s.Boards.Selected = in.Board.ID
		s.Notice = Notice{Text: "Added dashboard " + in.Board.Name}
		return s, []Command{BoardsFetch{}}
	case WidgetAdd:
		s.Errors = without(s.Errors, FieldWidgetPhase, FieldWidgetTables)
		b, ok := s.board(s.Boards.Selected)
		if !ok {
			s.Notice = Notice{Text: "Add a dashboard first", Failed: true}
			return s, nil
		}
		tables, err := strconv.Atoi(strings.TrimSpace(s.Fields[FieldWidgetTables]))
		if err != nil {
			s.Errors = with(s.Errors, FieldWidgetTables, "the size is a whole number of tables")
			return s, nil
		}
		position := 1
		for _, w := range b.Widgets {
			position = max(position, w.Position+1)
		}
		return s, []Command{WidgetPost{Board: b.ID, Position: position, Phase: db.Phase(s.Fields[FieldWidgetPhase]), Tables: tables}}
	case WidgetPosted:
		if in.Err != nil {
			s, _ = formFailed(s, in.Err, widgetForm)
			return s, nil
		}
		s.Notice = Notice{Text: "Added " + string(in.Widget.Phase) + " at " + strconv.Itoa(in.Widget.Tables) + " tables"}
		return s, []Command{BoardsFetch{}}
	case WidgetRemove:
		if s.Boards.Selected == 0 {
			return s, nil
		}
		return s, []Command{WidgetDelete{Board: s.Boards.Selected, ID: in.ID}}
	case WidgetDeleted:
		if in.Err != nil {
			return failed(s, "removing widget "+strconv.Itoa(in.ID), in.Err), nil
		}
		s.Notice = Notice{Text: "Removed widget " + strconv.Itoa(in.ID)}
		return s, []Command{BoardsFetch{}}
	}
	return s, nil
}

func screenShow(s State, to Screen) (State, []Command) {
	if s.User.ID == 0 {
		s.Screen = ScreenSignIn
		return s, []Command{UsersFetch{}}
	}
	s.Screen = to
	switch to {
	case ScreenRuns:
		return s, []Command{runsFetch(s), TagsFetch{}}
	case ScreenRun:
		if s.Run.ID == 0 {
			return s, nil
		}
		return s, []Command{RunFetch{ID: s.Run.ID}, TagsFetch{}}
	case ScreenTrends:
		if s.Trend.Tables == 0 {
			s.Trend = Spot{Phase: db.PhaseLoad, Tables: TablesDefault}
		}
		return s, []Command{TrendFetch{Spot: s.Trend}}
	case ScreenThesis:
		if s.Thesis.Fixture == "" {
			s.Thesis.Fixture = Fixtures[0]
		}
		return s, []Command{ThesisFetch{Fixture: s.Thesis.Fixture}}
	case ScreenBudgets:
		if s.Fields[FieldBudgetPhase] == "" {
			s.Fields = with(s.Fields, FieldBudgetPhase, string(db.PhaseLoad))
		}
		return s, budgetsFetch(s)
	case ScreenDashboards:
		if s.Fields[FieldWidgetPhase] == "" {
			s.Fields = with(s.Fields, FieldWidgetPhase, string(db.PhaseLoad))
		}
		return s, []Command{BoardsFetch{}}
	}
	return s, nil
}

func changed(s State, c Changed) (State, []Command) {
	s.Live = c.Handler + " " + c.Path
	if s.User.ID == 0 {
		return s, nil
	}
	var cmds []Command
	switch {
	case c.Handler == "Uploads.Create":
		cmds = append(cmds, runsFetch(s))
		cmds = append(cmds, budgetsFetch(s)...)
		if s.Screen == ScreenTrends {
			cmds = append(cmds, TrendFetch{Spot: s.Trend})
		}
		if s.Boards.Loaded {
			cmds = append(cmds, BoardsFetch{})
		}
	case strings.HasPrefix(c.Handler, "Runs.") || c.Handler == "Samples.Create":
		cmds = append(cmds, runsFetch(s))
		if s.Run.ID != 0 {
			cmds = append(cmds, RunFetch{ID: s.Run.ID})
		}
	case strings.HasPrefix(c.Handler, "db.Tag"):
		cmds = append(cmds, TagsFetch{}, runsFetch(s))
	case strings.HasPrefix(c.Handler, "Budgets."):
		cmds = append(cmds, budgetsFetch(s)...)
	case c.Handler == "Dashboards.Create" || strings.HasPrefix(c.Handler, "Widgets."):
		cmds = append(cmds, BoardsFetch{})
	case c.Handler == "Thesis.Record":
		if s.Thesis.Fixture != "" {
			cmds = append(cmds, ThesisFetch{Fixture: s.Thesis.Fixture})
		}
	}
	return s, cmds
}

// ChangedOf is the intent an event of the server's stream carries, and
// false for an event the client does not know.
func ChangedOf(ev volt.Event) (Intent, bool) {
	if ev.Name != "changed" {
		return nil, false
	}
	var c Changed
	if err := json.Unmarshal(ev.Data, &c); err != nil {
		return nil, false
	}
	return c, true
}

func runsFetch(s State) RunsFetch {
	return RunsFetch{Tag: s.Runs.Tag, After: s.Runs.After, Limit: RunsLimit}
}

func budgetsFetch(s State) []Command {
	cmds := []Command{BudgetsFetch{}}
	if s.Budgets.Selected != 0 {
		cmds = append(cmds, BudgetFetch{ID: s.Budgets.Selected})
	}
	return cmds
}

func (s State) budget(id int) (db.Budget, bool) {
	i := slices.IndexFunc(s.Budgets.List, func(b db.Budget) bool { return b.ID == id })
	if i < 0 {
		return db.Budget{}, false
	}
	return s.Budgets.List[i], true
}

func (s State) board(id int) (db.DashboardWithWidgets, bool) {
	i := slices.IndexFunc(s.Boards.List, func(b db.DashboardWithWidgets) bool { return b.ID == id })
	if i < 0 {
		return db.DashboardWithWidgets{}, false
	}
	return s.Boards.List[i], true
}

// BoardSelected is the dashboard the dashboards screen shows.
func (s State) BoardSelected() (db.DashboardWithWidgets, bool) { return s.board(s.Boards.Selected) }

// BudgetSelected is the budget the budgets screen edits.
func (s State) BudgetSelected() (db.Budget, bool) { return s.budget(s.Budgets.Selected) }

// Sizes are the table counts the open run measured, ascending, or the
// default size when no run is open.
func (s State) Sizes() []int {
	var out []int
	for _, p := range s.Run.Detail.Phases {
		if !slices.Contains(out, p.Tables) {
			out = append(out, p.Tables)
		}
	}
	if s.Trend.Tables != 0 && !slices.Contains(out, s.Trend.Tables) {
		out = append(out, s.Trend.Tables)
	}
	if len(out) == 0 {
		out = append(out, TablesDefault)
	}
	slices.Sort(out)
	return out
}

func failed(s State, doing string, err error) State {
	s.Notice = Notice{Text: doing + ": " + problemText(err), Failed: true}
	return s
}

func formFailed(s State, err error, f form) (State, bool) {
	var pe *volt.ProblemError
	placed := false
	if errors.As(err, &pe) {
		for _, d := range pe.Details() {
			var fields []Field
			for _, col := range d.Columns {
				if field, ok := f.columns[col]; ok {
					fields = append(fields, field)
				}
			}
			if field, ok := f.checks[d.Check]; ok && len(d.Columns) == 0 {
				fields = append(fields, field)
			}
			for _, field := range fields {
				msg := d.Message
				if prev := s.Errors[field]; prev != "" && prev != msg {
					msg = prev + "; " + msg
				}
				s.Errors = with(s.Errors, field, msg)
				placed = true
			}
		}
	}
	s.Notice = Notice{Text: problemText(err), Failed: true}
	return s, placed
}

func problemText(err error) string {
	var pe *volt.ProblemError
	if errors.As(err, &pe) {
		return strconv.Itoa(pe.Status) + " " + pe.Message
	}
	return err.Error()
}

func with[K comparable, V any](m map[K]V, k K, v V) map[K]V {
	out := maps.Clone(m)
	if out == nil {
		out = map[K]V{}
	}
	out[k] = v
	return out
}

func without[K comparable, V any](m map[K]V, ks ...K) map[K]V {
	out := maps.Clone(m)
	for _, k := range ks {
		delete(out, k)
	}
	return out
}
