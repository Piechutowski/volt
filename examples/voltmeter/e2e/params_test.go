package e2e

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

// TestListAndPrefixParameters proves how a query route binds what is
// not a plain value (spec §query-routes, §dataset-declaration,
// §generated-client): a list parameter from the repeated query key and
// as the empty list when absent, a dataset parameter from a scope
// prefix of the same name, a dataset narrowed by only:, and a value
// that does not parse as its type refused with a 400, through the
// generated client and through raw HTTP alike.
func TestListAndPrefixParameters(t *testing.T) {
	e := envStart(t)
	c := e.gob(0)
	run := runUpload(t, c, sweep("ab010001", []int{2, 4, 8}, 1)).Run
	runUpload(t, c, sweep("ab010002", []int{4}, 2))

	tablesOf := func(rows []db.MsLoad) []int {
		var out []int
		for _, r := range rows {
			out = append(out, r.Tables)
		}
		return out
	}
	rows, err := c.APISizedMsLoadSized(bg, run.ID, []int{8, 2})
	if err != nil || !slices.Equal(tablesOf(rows), []int{2, 8}) || rows[0].RunID != run.ID {
		t.Errorf("sized load of run %d at 8 and 2 = %v, %v", run.ID, tablesOf(rows), err)
	}
	var raw []db.MsLoad
	if r := e.rawJSON("GET", "/api/runs/1/sized/load?sizes=8&sizes=2", "", &raw); r.StatusCode != 200 || !slices.Equal(tablesOf(raw), []int{2, 8}) {
		t.Errorf("repeated sizes keys over raw HTTP = %d %s", r.StatusCode, r.body)
	}
	if rows, err := c.APISizedMsLoadSized(bg, run.ID, nil); err != nil || len(rows) != 0 {
		t.Errorf("sized with no sizes = %v, %v; an absent list is the empty list", rows, err)
	}
	if r := e.raw("GET", "/api/runs/1/sized/load", ""); r.StatusCode != 200 || string(r.body) != "[]\n" {
		t.Errorf("sized without the key over raw HTTP = %d %q", r.StatusCode, r.body)
	}
	if gen, err := c.APISizedMsGenerateSized(bg, run.ID, []int{4}); err != nil || len(gen) != 1 || gen[0].OutputMb != 0.4 {
		t.Errorf("sized generate = %+v, %v", gen, err)
	}
	for path, want := range map[string]int{
		"/api/runs/1/sized/check":          http.StatusNotFound,
		"/api/runs/x/sized/load":           http.StatusNotFound,
		"/api/runs/1/sized/load?sizes=two": http.StatusBadRequest,
		"/api/trend/load?tables=x":         http.StatusBadRequest,
		"/api/per_size/vet?sizes=4&sizes=": http.StatusBadRequest,
	} {
		if r := e.raw("GET", path, ""); r.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, r.StatusCode, want)
		}
	}

	per, err := c.APIPerSizeMsLoadPerSize(bg, []int{4, 8})
	if err != nil || len(per) != 2 || per[0].Tables != 4 || per[0].N != 2 || per[1].Tables != 8 || per[1].N != 1 {
		t.Errorf("per size of load at 4 and 8 = %+v, %v", per, err)
	}
	if per, err := c.APIPerSizeMsVetPerSize(bg, nil); err != nil || len(per) != 0 {
		t.Errorf("per size with no sizes = %+v, %v", per, err)
	}
	sizes, err := c.APISizes(bg)
	if err != nil || !slices.Equal(sizes, []db.Sizes{{Tables: 2, Runs: 1}, {Tables: 4, Runs: 2}, {Tables: 8, Runs: 1}}) {
		t.Errorf("sizes = %+v, %v", sizes, err)
	}
}

// TestRemovedActionsAreRefused proves the actions only:, except: and api
// leave out of a resources declaration are not routed (spec
// §the-action-table, §settings): a method the path serves for another
// action is a 405 naming the methods it allows, and a member path of a
// resource routed only as a collection is a 404.
func TestRemovedActionsAreRefused(t *testing.T) {
	e := envStart(t)
	for _, c := range []struct {
		method, path string
		status       int
		allow        string
	}{
		{"POST", "/api/machines", 405, "GET, HEAD"},
		{"PATCH", "/api/machines/1", 405, "GET, HEAD"},
		{"DELETE", "/api/budgets/1", 405, "GET, HEAD, PATCH"},
		{"POST", "/api/runs", 405, "GET, HEAD, OPTIONS"},
		{"PUT", "/api/runs/1", 405, "DELETE, GET, HEAD, PATCH"},
		{"GET", "/api/users/1", 404, ""},
		{"POST", "/api/users", 405, "GET, HEAD"},
	} {
		r := e.raw(c.method, c.path, "", "X-User", "1")
		if r.StatusCode != c.status || r.Header.Get("Allow") != c.allow {
			t.Errorf("%s %s = %d, Allow %q; want %d, Allow %q", c.method, c.path, r.StatusCode, r.Header.Get("Allow"), c.status, c.allow)
		}
	}
}
