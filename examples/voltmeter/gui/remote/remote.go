// Package remote performs the Voltmeter client's commands over the
// generated client in GOB and answers each with the result intent the
// state package defines (spec §generated-client, §values-and-effects):
// it is the client's one edge, beside the event stream main owns. It
// links no server code: a controller route's raw client method is read
// with replyRead, and the two requests no client method makes, the run
// list with its query string and a budget create, spell their path.
package remote

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app/client"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
	"github.com/Piechutowski/volt/nao/rt"
)

// Remote is one client's connection to a Voltmeter server: the
// generated client in GOB with a cookie jar of its own, so the sign-in
// cookie names the member on every later call, and the directory CSV
// exports are saved to.
type Remote struct {
	C      *client.Client
	Export string

	mu  sync.Mutex
	jar http.CookieJar
}

// New is a Remote speaking to base and saving exports under export.
func New(base, export string) *Remote {
	r := &Remote{C: client.New(base), Export: export}
	r.C.Format = volt.FormatGOB
	r.jar, _ = cookiejar.New(nil)
	r.C.HTTP = &http.Client{Jar: jarOf{r}}
	return r
}

type jarOf struct{ r *Remote }

func (j jarOf) SetCookies(u *url.URL, cs []*http.Cookie) {
	j.r.mu.Lock()
	defer j.r.mu.Unlock()
	j.r.jar.SetCookies(u, cs)
}

func (j jarOf) Cookies(u *url.URL) []*http.Cookie {
	j.r.mu.Lock()
	defer j.r.mu.Unlock()
	return j.r.jar.Cookies(u)
}

// Events is the server's event stream, reconnecting until ctx is done
// (spec §event-routes).
func (r *Remote) Events(ctx context.Context) <-chan volt.Event { return r.C.Events(ctx) }

// Perform makes the request cmd describes and returns the intent that
// answers it; CookiesDrop answers nothing and returns nil.
func (r *Remote) Perform(ctx context.Context, cmd state.Command) state.Intent {
	c := r.C
	switch cmd := cmd.(type) {
	case state.UsersFetch:
		users, err := c.APIUsers(ctx)
		return state.UsersFetched{Users: users, Err: err}
	case state.SignInPost:
		var u db.User
		resp, err := c.SignIn(ctx, signIn{Email: cmd.Email})
		err = replyRead(resp, err, &u)
		return state.SignedIn{User: u, Err: err}
	case state.CookiesDrop:
		jar, _ := cookiejar.New(nil)
		r.mu.Lock()
		r.jar = jar
		r.mu.Unlock()
		return nil
	case state.RunsFetch:
		q := []volt.URLOption{volt.Query("limit", strconv.Itoa(cmd.Limit))}
		if cmd.After != 0 {
			q = append(q, volt.Query("after", strconv.Itoa(cmd.After)))
		}
		if cmd.Tag != "" {
			q = append(q, volt.Query("tag", cmd.Tag))
		}
		var page wire.RunPage
		err := c.Do(ctx, "GET", volt.URL("/api/runs", q...), nil, &page)
		return state.RunsFetched{Query: cmd, Page: page, Err: err}
	case state.RunFetch:
		var d wire.RunDetail
		resp, err := c.APIRun(ctx, cmd.ID)
		err = replyRead(resp, err, &d)
		return state.RunFetched{ID: cmd.ID, Detail: d, Err: err}
	case state.RunPatch:
		cs := db.RunNew().TagSet(nullOf(cmd.Tag)).NoteSet(nullOf(cmd.Note))
		var run db.Run
		resp, err := c.APIRunAnnotate(ctx, cmd.ID, cs)
		err = replyRead(resp, err, &run)
		return state.RunPatched{Run: run, Err: err}
	case state.RunCSVSave:
		path, n, err := r.csvSave(ctx, cmd.ID)
		return state.RunCSVSaved{ID: cmd.ID, Path: path, Size: n, Err: err}
	case state.TagsFetch:
		tags, err := c.APITags(ctx)
		return state.TagsFetched{Tags: tags, Err: err}
	case state.TagPost:
		tag, err := c.APICreateTag(ctx, db.TagNew().SlugSet(cmd.Slug).NameSet(cmd.Name))
		return state.TagSaved{Tag: tag, Err: err}
	case state.TagPatch:
		tag, err := c.APIUpdateTag(ctx, cmd.Slug, db.TagNew().NameSet(cmd.Name))
		return state.TagSaved{Tag: tag, Err: err}
	case state.TagDelete:
		return state.TagDeleted{Slug: cmd.Slug, Err: c.APIDeleteTag(ctx, cmd.Slug)}
	case state.SizesFetch:
		rows, err := c.APISizes(ctx)
		sizes := make([]int, 0, len(rows))
		for _, r := range rows {
			sizes = append(sizes, r.Tables)
		}
		return state.SizesFetched{Sizes: sizes, Err: err}
	case state.TrendFetch:
		points, err := r.trend(ctx, cmd.Spot)
		return state.TrendFetched{Spot: cmd.Spot, Points: points, Err: err}
	case state.ThesisFetch:
		rows, err := c.APILineCountThesis(ctx, cmd.Fixture)
		return state.ThesisFetched{Fixture: cmd.Fixture, Rows: rows, Err: err}
	case state.BudgetsFetch:
		var bs []db.Budget
		resp, err := c.APIBudgets(ctx)
		err = replyRead(resp, err, &bs)
		return state.BudgetsFetched{Budgets: bs, Err: err}
	case state.BudgetFetch:
		var d wire.BudgetDetail
		resp, err := c.APIBudget(ctx, cmd.ID)
		err = replyRead(resp, err, &d)
		return state.BudgetFetched{ID: cmd.ID, Detail: d, Err: err}
	case state.BudgetPatch:
		var b db.Budget
		resp, err := c.APIBudgetCeiling(ctx, cmd.ID, db.BudgetNew().MaxMsSet(cmd.MaxMS))
		err = replyRead(resp, err, &b)
		return state.BudgetPatched{Budget: b, Err: err}
	case state.BudgetPost:
		var b db.Budget
		cs := db.BudgetNew().PhaseSet(cmd.Phase).TablesSet(cmd.Tables).MaxMsSet(cmd.MaxMS)
		err := c.Do(ctx, "POST", volt.URL("/api/budgets"), cs.Params(), &b)
		return state.BudgetPosted{Budget: b, Err: err}
	case state.BudgetsResetPost:
		var n wire.Reset
		resp, err := c.APIBudgetsReset(ctx)
		err = replyRead(resp, err, &n)
		return state.BudgetsResetPosted{Reset: n, Err: err}
	case state.BoardsFetch:
		var bs []db.DashboardWithWidgets
		resp, err := c.APIMeDashboards(ctx)
		err = replyRead(resp, err, &bs)
		return state.BoardsFetched{Boards: bs, Err: err}
	case state.BoardPost:
		var d db.Dashboard
		resp, err := c.APIMeDashboardAdd(ctx, db.DashboardNew().NameSet(cmd.Name))
		err = replyRead(resp, err, &d)
		return state.BoardPosted{Board: d, Err: err}
	case state.WidgetPost:
		var w db.Widget
		cs := db.WidgetNew().PositionSet(cmd.Position).PhaseSet(cmd.Phase).TablesSet(cmd.Tables)
		resp, err := c.APIMeBoardWidgetAdd(ctx, cmd.Board, cs)
		err = replyRead(resp, err, &w)
		return state.WidgetPosted{Widget: w, Err: err}
	case state.WidgetDelete:
		resp, err := c.APIMeBoardWidgetRemove(ctx, cmd.Board, cmd.ID)
		err = replyRead(resp, err, nil)
		return state.WidgetDeleted{ID: cmd.ID, Err: err}
	}
	panic(fmt.Sprintf("remote: no request performs %T", cmd))
}

func (r *Remote) trend(ctx context.Context, spot state.Spot) ([]state.TrendPoint, error) {
	c := r.C
	var points []state.TrendPoint
	var err error
	switch spot.Phase {
	case db.PhaseLoad:
		var rows []db.MsLoad
		rows, err = c.APITrendMsLoadTrend(ctx, spot.Tables)
		for _, m := range rows {
			points = append(points, state.TrendPoint{RunID: m.RunID, WallMS: m.WallMs})
		}
	case db.PhaseCheck:
		var rows []db.MsCheck
		rows, err = c.APITrendMsCheckTrend(ctx, spot.Tables)
		for _, m := range rows {
			points = append(points, state.TrendPoint{RunID: m.RunID, WallMS: m.WallMs})
		}
	case db.PhaseVet:
		var rows []db.MsVet
		rows, err = c.APITrendMsVetTrend(ctx, spot.Tables)
		for _, m := range rows {
			points = append(points, state.TrendPoint{RunID: m.RunID, WallMS: m.WallMs})
		}
	case db.PhaseGenerate:
		var rows []db.MsGenerate
		rows, err = c.APITrendMsGenerateTrend(ctx, spot.Tables)
		for _, m := range rows {
			points = append(points, state.TrendPoint{RunID: m.RunID, WallMS: m.WallMs})
		}
	default:
		err = fmt.Errorf("%q is not a phase", spot.Phase)
	}
	return points, err
}

func (r *Remote) csvSave(ctx context.Context, id int) (string, int, error) {
	resp, err := r.C.ReportExport(ctx, id)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, volt.Error(resp.StatusCode, "the export answered "+resp.Status)
	}
	if err := os.MkdirAll(r.Export, 0o755); err != nil {
		return "", 0, err
	}
	path := filepath.Join(r.Export, "voltmeter-run-"+strconv.Itoa(id)+".csv")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", 0, err
	}
	return path, len(body), nil
}

type signIn struct {
	Email string `json:"email"`
}

func replyRead(resp *http.Response, err error, out any) error {
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, known := volt.FormatOf(resp.Header.Get("Content-Type"))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var p volt.Problem
		if known && volt.DecodeBody(f, io.LimitReader(resp.Body, 1<<20), &p) == nil && p.Status != 0 {
			return &volt.ProblemError{Problem: p}
		}
		return volt.Error(resp.StatusCode, resp.Status)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if !known {
		return fmt.Errorf("%s %s answered %q, which is neither JSON nor GOB", resp.Request.Method, resp.Request.URL.Path, resp.Header.Get("Content-Type"))
	}
	return volt.DecodeBody(f, resp.Body, out)
}

func nullOf(s string) rt.Null[string] {
	if s == "" {
		return rt.Null[string]{}
	}
	return rt.Some(s)
}
