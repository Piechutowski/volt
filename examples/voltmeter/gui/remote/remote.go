// Package remote performs the Voltmeter client's commands over the
// generated client in GOB and answers each with the result intent the
// state package defines (spec §generated-client, §values-and-effects):
// it is the client's one edge, beside the event stream main owns.
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
	"github.com/Piechutowski/volt/examples/voltmeter/app"
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
		err := c.Do(ctx, "POST", app.PathSignIn(), app.SignIn{Email: cmd.Email}, &u)
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
		err := c.Do(ctx, "GET", app.PathAPIRuns(q...), nil, &page)
		return state.RunsFetched{Query: cmd, Page: page, Err: err}
	case state.RunFetch:
		var d wire.RunDetail
		err := c.Do(ctx, "GET", app.PathAPIRun(cmd.ID), nil, &d)
		return state.RunFetched{ID: cmd.ID, Detail: d, Err: err}
	case state.RunPatch:
		cs := db.RunNew().TagSet(nullOf(cmd.Tag)).NoteSet(nullOf(cmd.Note))
		var run db.Run
		err := c.Do(ctx, "PATCH", app.PathAPIRunAnnotate(cmd.ID), cs.Params(), &run)
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
		err := c.Do(ctx, "GET", app.PathAPIBudgets(), nil, &bs)
		return state.BudgetsFetched{Budgets: bs, Err: err}
	case state.BudgetFetch:
		var d wire.BudgetDetail
		err := c.Do(ctx, "GET", app.PathAPIBudget(cmd.ID), nil, &d)
		return state.BudgetFetched{ID: cmd.ID, Detail: d, Err: err}
	case state.BudgetPatch:
		var b db.Budget
		err := c.Do(ctx, "PATCH", app.PathAPIBudgetCeiling(cmd.ID), db.BudgetNew().MaxMsSet(cmd.MaxMS).Params(), &b)
		return state.BudgetPatched{Budget: b, Err: err}
	case state.BudgetPost:
		var b db.Budget
		cs := db.BudgetNew().PhaseSet(cmd.Phase).TablesSet(cmd.Tables).MaxMsSet(cmd.MaxMS)
		err := c.Do(ctx, "POST", app.PathAPIBudgets(), cs.Params(), &b)
		return state.BudgetPosted{Budget: b, Err: err}
	case state.BudgetsResetPost:
		var n wire.Reset
		err := c.Do(ctx, "POST", app.PathAPIBudgetsReset(), nil, &n)
		return state.BudgetsResetPosted{Reset: n, Err: err}
	case state.BoardsFetch:
		var bs []db.DashboardWithWidgets
		err := c.Do(ctx, "GET", app.PathAPIMeDashboards(), nil, &bs)
		return state.BoardsFetched{Boards: bs, Err: err}
	case state.BoardPost:
		var d db.Dashboard
		err := c.Do(ctx, "POST", app.PathAPIMeDashboardAdd(), db.DashboardNew().NameSet(cmd.Name).Params(), &d)
		return state.BoardPosted{Board: d, Err: err}
	case state.WidgetPost:
		var w db.Widget
		cs := db.WidgetNew().PositionSet(cmd.Position).PhaseSet(cmd.Phase).TablesSet(cmd.Tables)
		err := c.Do(ctx, "POST", app.PathAPIMeBoardWidgetAdd(cmd.Board), cs.Params(), &w)
		return state.WidgetPosted{Widget: w, Err: err}
	case state.WidgetDelete:
		err := c.Do(ctx, "DELETE", app.PathAPIMeBoardWidgetRemove(cmd.Board, cmd.ID), nil, nil)
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

func nullOf(s string) rt.Null[string] {
	if s == "" {
		return rt.Null[string]{}
	}
	return rt.Some(s)
}
