package e2e

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/wire"
)

// TestRouteLoadsBindFromTheQuery proves the binding rules of a load
// block beyond the path (spec §route-loads, rule 2): a load argument
// that is no path parameter binds from the query string, a list one from
// the repeated key, a missing or unparseable value is a 400 naming the
// key before any load runs, and a load's row miss is the spine's 404.
func TestRouteLoadsBindFromTheQuery(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	base := runUpload(t, c, sweep("ad000001", []int{2, 4, 8}, 1)).Run
	next := runUpload(t, c, sweep("ad000002", []int{2, 4}, 2)).Run

	var cmp wire.Comparison
	r := e.rawJSON("GET", "/api/runs/2/compare?baseline=1&sizes=4&sizes=2&sizes=8", "", &cmp)
	if r.StatusCode != 200 || cmp.Run.ID != next.ID || cmp.Against.ID != base.ID {
		t.Fatalf("compare = %d %s", r.StatusCode, r.body)
	}
	want := []wire.Delta{{Tables: 2, Mine: 4, Theirs: 2, Change: 100}, {Tables: 4, Mine: 8, Theirs: 4, Change: 100}}
	if !slices.Equal(cmp.Sizes, want) {
		t.Errorf("compare sizes = %+v, want %+v", cmp.Sizes, want)
	}
	if r := e.rawJSON("GET", app.PathAPIRunCompare(next.ID, volt.Query("baseline", "1")), "", &cmp); r.StatusCode != 200 || len(cmp.Sizes) != 0 {
		t.Errorf("compare with no sizes = %d %+v; an absent list is empty", r.StatusCode, cmp.Sizes)
	}
	for path, want := range map[string]int{
		"/api/runs/2/compare":                    http.StatusBadRequest,
		"/api/runs/2/compare?baseline=one":       http.StatusBadRequest,
		"/api/runs/99/compare?baseline=one":      http.StatusBadRequest,
		"/api/runs/2/compare?baseline=1&sizes=x": http.StatusBadRequest,
		"/api/runs/2/compare?baseline=99":        http.StatusNotFound,
		"/api/runs/99/compare?baseline=1":        http.StatusNotFound,
	} {
		r := e.raw("GET", path, "")
		if r.StatusCode != want {
			t.Errorf("GET %s = %d, want %d: %s", path, r.StatusCode, want, r.body)
		}
		if want == http.StatusBadRequest && r.Header.Get("X-Error-Handler") != "api" {
			t.Errorf("GET %s answered without the api error handler", path)
		}
	}
}

// TestQueryRouteUnderAScopeLoad proves a query route beneath a scope
// load runs the load for its halt and drops its value (spec
// §scope-loads, rule 3): no member is the load's 401, a member who is
// no row is its 404, and a member sees the boards of the owner the
// query names, not their own.
func TestQueryRouteUnderAScopeLoad(t *testing.T) {
	e := envStart(t)
	var boards []db.DashboardWithWidgets
	if r := e.rawJSON("POST", app.PathAPIMeDashboardAdd(), `{"name": "Grace's"}`, nil, "X-User", "2"); r.StatusCode != http.StatusCreated {
		t.Fatalf("Grace's board = %d %s", r.StatusCode, r.body)
	}
	path := app.PathAPIMeBoardsOf(volt.Query("owner", "2"))
	if r := e.raw("GET", path, ""); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("no member = %d", r.StatusCode)
	}
	if r := e.raw("GET", path, "", "X-User", "77"); r.StatusCode != http.StatusNotFound {
		t.Errorf("a member who is no row = %d", r.StatusCode)
	}
	r := e.rawJSON("GET", path, "", &boards, "X-User", "1")
	if r.StatusCode != 200 || len(boards) != 1 || boards[0].Name != "Grace's" || boards[0].OwnerID != 2 || r.Header.Get("Cache-Control") != "private" {
		t.Errorf("Ada reading Grace's boards = %d %+v %v", r.StatusCode, boards, r.Header)
	}
}

// TestMalformedBodiesBeforeTheRow proves a body is decoded before any
// row loads (spec §request-bodies, rule 5; §query-routes): a malformed
// body is a 400 whether or not the row exists, on a controller's
// changeset body and on a generated update alike.
func TestMalformedBodiesBeforeTheRow(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	runUpload(t, c, sweep("ae000001", []int{2}, 1))
	if r := e.raw("POST", app.PathAPIBudgets(), `{"phase": "load", "tables": 2, "max_ms": 5}`); r.StatusCode != http.StatusCreated {
		t.Fatalf("budget = %d", r.StatusCode)
	}
	for _, path := range []string{app.PathAPIRunAnnotate(1), app.PathAPIRunAnnotate(99), app.PathAPITag("baseline"), app.PathAPITag("absent"), app.PathAPIBudgetCeiling(1), app.PathAPIBudgetCeiling(99)} {
		if r := e.raw("PATCH", path, `{`); r.StatusCode != http.StatusBadRequest {
			t.Errorf("PATCH %s with a malformed body = %d, want 400", path, r.StatusCode)
		}
	}
}
