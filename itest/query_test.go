// Query routes (spec §query-routes, §formats): the generated handlers bind
// parameters from path, body and query string, call the data package's
// queries against a float64 SQLite, and render in the negotiated format.
// Every status the spec names is served here, in both formats.
package itest

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/db"
)

func call(h http.Handler, method, target string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder, status int) volt.Problem {
	t.Helper()
	var p volt.Problem
	if rec.Code != status || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Status != status {
		t.Fatalf("%d %q, want %d with a Problem body", rec.Code, rec.Body.String(), status)
	}
	return p
}

func detailNamed(details []volt.Detail, check string, columns ...string) bool {
	for _, d := range details {
		if d.Check == check && slices.Equal(d.Columns, columns) {
			return true
		}
	}
	return false
}

func TestQueryRouteCRUDInJSON(t *testing.T) {
	h := handlerWithDB(t)

	rec := call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var made db.User
	if err := json.Unmarshal(rec.Body.Bytes(), &made); err != nil || made.Email != "two@example.com" || made.ID == 0 {
		t.Fatalf("create body %q (%v)", rec.Body.String(), err)
	}

	rec = call(h, "GET", app.PathAPIUserList(), nil)
	var all []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &all) != nil || len(all) != 2 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, "GET", app.PathAPIUserGet(made.ID), nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "two@example.com") {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathAPIUserGet(999), nil); rec.Code != 404 {
		t.Fatalf("get miss: %d, want 404", rec.Code)
	}
	if rec = call(h, "GET", "/api/users/abc", nil); rec.Code != 404 {
		t.Fatalf("get with a non-int id: %d, want 404 (§route-paths)", rec.Code)
	}

	rec = call(h, "PATCH", "/api/users/1", strings.NewReader(`{"email":"uno@example.com"}`), "Content-Type", volt.MIMEJSON)
	var updated db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &updated) != nil || updated.ID != 1 || updated.Email != "uno@example.com" {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, "PATCH", "/api/users/1", strings.NewReader(`{}`))
	var same db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &same) != nil || same != updated {
		t.Fatalf("empty update: %d %s, want 200 with the unchanged row (§changesets)", rec.Code, rec.Body.String())
	}
	if rec = call(h, "PATCH", "/api/users/999", strings.NewReader(`{"email":"x@example.com"}`)); rec.Code != 404 {
		t.Fatalf("update miss: %d, want 404", rec.Code)
	}

	if rec = call(h, "DELETE", "/api/users/1", nil); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", rec.Code, rec.Body.String())
	}
	if rec = call(h, "DELETE", "/api/users/1", nil); rec.Code != 404 {
		t.Fatalf("delete miss: %d, want 404", rec.Code)
	}
}

func TestQueryRouteListParamFromRepeatedKey(t *testing.T) {
	h := handlerWithDB(t)
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"three@example.com"}`))

	rec := call(h, "GET", app.PathAPIUserPicked(volt.Query("ids", "1"), volt.Query("ids", "3")), nil)
	var picked []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &picked) != nil {
		t.Fatalf("picked: %d %s", rec.Code, rec.Body.String())
	}
	if len(picked) != 2 || picked[0].ID != 1 || picked[1].ID != 3 {
		t.Fatalf("picked = %+v, want ids 1 and 3 in id order", picked)
	}
	rec = call(h, "GET", app.PathAPIUserPicked(), nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("picked with no ids: %d %q, want 200 []", rec.Code, rec.Body.String())
	}
	rec = call(h, "GET", "/api/picked?ids=1&ids=x", nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "ids") {
		t.Fatalf("bad list element: %d %q", rec.Code, rec.Body.String())
	}
}

func TestQueryRouteGOBBothWays(t *testing.T) {
	h := handlerWithDB(t)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(db.UserNew().EmailSet("gob@example.com").Params()); err != nil {
		t.Fatal(err)
	}
	rec := call(h, "POST", "/api/users", &buf, "Content-Type", volt.MIMEGOB, "Accept", volt.MIMEGOB)
	if rec.Code != 201 || rec.Header().Get("Content-Type") != volt.MIMEGOB {
		t.Fatalf("gob create: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var made db.User
	if err := gob.NewDecoder(rec.Body).Decode(&made); err != nil || made.Email != "gob@example.com" {
		t.Fatalf("gob create body: %+v (%v)", made, err)
	}

	rec = call(h, "GET", app.PathAPIUserList(), nil, "Accept", volt.MIMEGOB)
	var all []db.User
	if err := gob.NewDecoder(rec.Body).Decode(&all); err != nil || len(all) != 2 {
		t.Fatalf("gob list: %d %+v (%v)", rec.Code, all, err)
	}
}

func TestQueryRouteFormatErrors(t *testing.T) {
	h := handlerWithDB(t)
	if rec := call(h, "GET", app.PathAPIUserList(), nil, "Accept", "text/html"); rec.Code != 406 {
		t.Errorf("unacceptable Accept: %d, want 406", rec.Code)
	}
	if rec := call(h, "POST", "/api/users", strings.NewReader("email=x"), "Content-Type", "application/x-www-form-urlencoded"); rec.Code != 415 {
		t.Errorf("form body: %d, want 415", rec.Code)
	}
	if rec := call(h, "POST", "/api/users", nil); rec.Code != 400 {
		t.Errorf("empty body: %d, want 400", rec.Code)
	}
	if rec := call(h, "POST", "/api/users", strings.NewReader("{oops")); rec.Code != 400 {
		t.Errorf("malformed body: %d, want 400", rec.Code)
	}
	if rec := call(h, "POST", "/api/users", strings.NewReader(`["x@example.com"]`)); rec.Code != 400 {
		t.Errorf("a body that is not an object: %d, want 400 (§changesets)", rec.Code)
	}
	if rec := call(h, "PATCH", "/api/users/999", strings.NewReader("{oops")); rec.Code != 400 {
		t.Errorf("malformed body for a missing row: %d, want 400; the body is decoded before the row is loaded (§query-routes)", rec.Code)
	}
	if rec := call(h, "POST", "/api/users", strings.NewReader(`{"email":"html@example.com"}`), "Accept", "text/html"); rec.Code != 406 {
		t.Errorf("unacceptable Accept on a write: %d, want 406", rec.Code)
	}
	var all []db.User
	if rec := call(h, "GET", app.PathAPIUserList(), nil); rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &all) != nil || len(all) != 1 {
		t.Errorf("a 406 write changed the table: %d %s; negotiation happens before anything runs (§formats)", rec.Code, rec.Body.String())
	}
}

// TestQueryRouteTimeParam (§query-routes, §generated-client): a time.Time
// parameter binds from the query string as RFC 3339, so the route and
// the client import time; an unparseable or missing value is a 400.
func TestQueryRouteTimeParam(t *testing.T) {
	h := handlerWithDB(t)
	rec := call(h, "GET", app.PathAPIUserSince(volt.Query("since", "2000-01-01T00:00:00Z")), nil)
	var rows []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rows) != nil || len(rows) != 1 || rows[0].CreatedAt.IsZero() {
		t.Fatalf("since 2000: %d %s, want the seeded row with its created_at", rec.Code, rec.Body.String())
	}
	rec = call(h, "GET", app.PathAPIUserSince(volt.Query("since", volt.FormatQuery(time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)))), nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("since 2999: %d %s, want 200 []", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", "/api/since?since=yesterday", nil); rec.Code != 400 || !strings.Contains(rec.Body.String(), "since") {
		t.Fatalf("unparseable time: %d %q, want 400 naming since", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", "/api/since", nil); rec.Code != 400 {
		t.Fatalf("missing time: %d, want 400", rec.Code)
	}
}

type statementCount struct{ n *atomic.Int64 }

func (h statementCount) Enabled(context.Context, slog.Level) bool { return true }

func (h statementCount) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "statement" {
		h.n.Add(1)
	}
	return nil
}

func (h statementCount) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h statementCount) WithGroup(string) slog.Handler      { return h }

// countedHandler is the fixture's router over a database that counts the
// statements it runs.
func countedHandler() (http.Handler, *atomic.Int64) {
	var n atomic.Int64
	dbh := fixtureDB().WithLogger(slog.New(statementCount{&n}))
	return app.NewRouter(fixtureControllers(dbh, &volt.Broker{})), &n
}

// TestQueryRouteLimitParam (§query-routes, §selects-over-groups): a
// select's limit parameter binds from the query string like any int,
// keeps the first rows in order, and a value below 1 is a 400 naming the
// parameter that runs no statement, as a missing or unparseable one is.
func TestQueryRouteLimitParam(t *testing.T) {
	h, ran := countedHandler()
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"three@example.com"}`))

	rec := call(h, "GET", app.PathAPIUserLatest(volt.Query("limit", "2")), nil)
	var rows []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rows) != nil || len(rows) != 2 || rows[0].ID != 3 || rows[1].ID != 2 {
		t.Fatalf("latest 2: %d %s, want ids 3 and 2", rec.Code, rec.Body.String())
	}
	for _, q := range []string{"limit=0", "limit=-1"} {
		var p volt.Problem
		before := ran.Load()
		rec = call(h, "GET", "/api/latest?"+q, nil)
		if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.Status != 400 || !strings.Contains(p.Message, "parameter limit") {
			t.Fatalf("%s: %d %q, want a 400 problem naming the parameter limit (§query-routes)", q, rec.Code, rec.Body.String())
		}
		if n := ran.Load() - before; n != 0 {
			t.Errorf("%s ran %d statements; want none, the limit refused where it binds", q, n)
		}
	}
	if rec = call(h, "GET", "/api/latest?limit=many", nil); rec.Code != 400 || !strings.Contains(rec.Body.String(), "limit") {
		t.Fatalf("unparseable limit: %d %q, want 400 naming limit", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", "/api/latest", nil); rec.Code != 400 {
		t.Fatalf("missing limit: %d, want 400", rec.Code)
	}
}

// TestLoadLimitParam (§route-loads, §selects-over-groups): a load binds a
// select's limit from a path parameter or the query string, and a value
// below 1 from either is a 400 naming its key before any load runs, the
// route's earlier load of the user included.
func TestLoadLimitParam(t *testing.T) {
	h, ran := countedHandler()
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"three@example.com"}`))

	served := ran.Load()
	rec := call(h, "GET", app.PathPeers(1, 2, volt.Query("count", "1")), nil)
	var got map[string]int
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got["user"] != 1 || got["peers"] != 2 || got["more"] != 1 {
		t.Fatalf("peers 2 and 1 more: %d %s, want user 1 with 2 peers and 1 more", rec.Code, rec.Body.String())
	}
	if n := ran.Load() - served; n != 3 {
		t.Fatalf("the served route ran %d statements; want its 3 loads, or the counts below prove nothing", n)
	}
	for _, tc := range []struct{ target, key string }{
		{app.PathPeers(1, 0, volt.Query("count", "1")), "parameter n"},
		{app.PathPeers(1, -1, volt.Query("count", "1")), "parameter n"},
		{app.PathPeers(1, 2, volt.Query("count", "0")), "parameter count"},
		{app.PathPeers(1, 2, volt.Query("count", "-1")), "parameter count"},
	} {
		var p volt.Problem
		before := ran.Load()
		rec = call(h, "GET", tc.target, nil)
		if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || !strings.Contains(p.Message, tc.key) {
			t.Errorf("%s: %d %q, want a 400 problem naming %s (§route-loads)", tc.target, rec.Code, rec.Body.String(), tc.key)
		}
		if n := ran.Load() - before; n != 0 {
			t.Errorf("%s ran %d statements; want none, every argument bound before the first load", tc.target, n)
		}
	}
}

// TestLoadKeyBindsTwice (§route-loads): one query key binds a required
// limit of one load and an optional parameter of another, each by its own
// rule, so the key present feeds both and the key missing is a 400 naming
// it before any load runs, the optional binding notwithstanding.
func TestLoadKeyBindsTwice(t *testing.T) {
	h, ran := countedHandler()
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"three@example.com"}`))

	rec := call(h, "GET", app.PathBoth(1, volt.Query("count", "3")), nil)
	var got map[string]int
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got["latest"] != 3 || got["described"] != 1 {
		t.Fatalf("count=3: %d %s, want the 3 latest and the 1 user from id 3", rec.Code, rec.Body.String())
	}
	before := ran.Load()
	var p volt.Problem
	rec = call(h, "GET", app.PathBoth(1), nil)
	if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || !strings.Contains(p.Message, "count") {
		t.Fatalf("no count: %d %q, want a 400 problem naming count", rec.Code, rec.Body.String())
	}
	if n := ran.Load() - before; n != 0 {
		t.Fatalf("no count ran %d statements; want none", n)
	}
}

// TestScopeLoadLimitParam (§scope-loads, §route-loads, §query-routes): a
// limit from the path, of a load or of a query route, is checked with the
// path parameters, before the scope load runs or halts, and one from the
// query string where it binds, after the scope load, as every query key is.
func TestScopeLoadLimitParam(t *testing.T) {
	h, ran := countedHandler()
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	call(h, "POST", "/api/users", strings.NewReader(`{"email":"three@example.com"}`))

	served := ran.Load()
	rec := call(h, "GET", app.PathMePicks(2), nil, "X-User", "1")
	var got map[string]int
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got["user"] != 1 || got["picks"] != 2 {
		t.Fatalf("picks 2: %d %s, want user 1 with 2 picks", rec.Code, rec.Body.String())
	}
	if n := ran.Load() - served; n != 2 {
		t.Fatalf("the served route ran %d statements; want the scope load's and the route load's, or the counts below prove nothing", n)
	}
	served = ran.Load()
	rec = call(h, "GET", app.PathMeTop(2), nil, "X-User", "1")
	var top []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &top) != nil || len(top) != 2 || top[0].ID != 3 || top[1].ID != 2 {
		t.Fatalf("top 2: %d %s, want users 3 and 2", rec.Code, rec.Body.String())
	}
	if n := ran.Load() - served; n != 2 {
		t.Fatalf("the served query route ran %d statements; want the scope load's and the query's, or the counts below prove nothing", n)
	}
	for _, user := range []string{"", "1"} {
		for _, n := range []int{0, -1} {
			for _, tc := range []struct{ target, key string }{
				{app.PathMePicks(n), "parameter n"},
				{app.PathMeTop(n), "parameter limit"},
			} {
				before := ran.Load()
				rec = call(h, "GET", tc.target, nil, "X-User", user)
				var p volt.Problem
				if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || !strings.Contains(p.Message, tc.key) {
					t.Errorf("%s, X-User %q: %d %q, want a 400 naming %s before the scope load (§scope-loads)", tc.target, user, rec.Code, rec.Body.String(), tc.key)
				}
				if count := ran.Load() - before; count != 0 {
					t.Errorf("%s, X-User %q ran %d statements; want none, the scope load's among them", tc.target, user, count)
				}
			}
		}
	}

	if rec = call(h, "GET", app.PathMeUserLatest(volt.Query("limit", "0")), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("latest limit=0 without a user: %d %q, want the scope load's 401, which runs before the query string binds", rec.Code, rec.Body.String())
	}
	before := ran.Load()
	rec = call(h, "GET", app.PathMeUserLatest(volt.Query("limit", "0")), nil, "X-User", "1")
	if p := problemOf(t, rec, 400); !strings.Contains(p.Message, "parameter limit") {
		t.Errorf("latest limit=0: %q, want a 400 naming limit", p.Message)
	}
	if n := ran.Load() - before; n != 1 {
		t.Errorf("latest limit=0 ran %d statements; want the scope load's alone, and no query", n)
	}
	rec = call(h, "GET", app.PathMeUserLatest(volt.Query("limit", "2")), nil, "X-User", "1")
	var rows []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &rows) != nil || len(rows) != 2 || rows[0].ID != 3 || rows[1].ID != 2 {
		t.Errorf("latest limit=2: %d %s, want users 3 and 2", rec.Code, rec.Body.String())
	}
}

// TestQueryRouteTable: query rows carry the reference as written and no
// controller, and only reads have helpers (§query-routes).
func TestQueryRouteTable(t *testing.T) {
	seen := 0
	for _, r := range app.Table {
		if r.Query == "" {
			continue
		}
		seen++
		if r.Controller != "" || r.Action != "" {
			t.Errorf("%s: query route carries a controller: %+v", r.Query, r)
		}
		if (r.Method == "GET") != (r.Helper != "") {
			t.Errorf("%s %s: helper %q — only GET query routes have one", r.Method, r.Spelled, r.Helper)
		}
	}
	if seen != 29 {
		t.Errorf("query rows = %d, want 29 (nine query routes, three under a scope load, two dataset members, fifteen resources [default] routes)", seen)
	}
}

// TestDatasetRoutesServe: the expanded routes serve the member's select
// with the select's parameters bound from the query string (§datasets).
func TestDatasetRoutesServe(t *testing.T) {
	h := handlerWithDB(t)
	for _, target := range []string{"/ms/revenue?year=2024", "/ms/usage?year=2024"} {
		rec := call(h, "GET", target, nil)
		if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("GET %s: %d %q, want 200 []", target, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, "GET", "/ms/revenue", nil); rec.Code != 400 || !strings.Contains(rec.Body.String(), "year") {
		t.Errorf("missing year: %d %q, want 400 naming year", rec.Code, rec.Body.String())
	}
	if rec := call(h, "GET", "/ms/notes?year=2024", nil); rec.Code != 404 {
		t.Errorf("a table outside the group: %d, want 404", rec.Code)
	}
}

// TestQueryRouteValidation (§validators, §query-routes, §changesets): a
// failed validator is a 422 whose details name it and its column,
// before the database sees the row; a duplicate key is a 409 with a
// unique detail on the index's column.
func TestQueryRouteValidation(t *testing.T) {
	h := handlerWithDB(t)

	p := problemOf(t, call(h, "POST", "/api/users", strings.NewReader(`{"email":"nope"}`)), 422)
	if !detailNamed(p.Details, "EmailValid", "email") {
		t.Fatalf("validator: details = %+v, want EmailValid on email", p.Details)
	}
	p = problemOf(t, call(h, "POST", "/api/users", strings.NewReader(`{"email":"@"}`)), 422)
	if !detailNamed(p.Details, "EmailShape", "email") {
		t.Fatalf("second validator: details = %+v, want EmailShape on email", p.Details)
	}
	p = problemOf(t, call(h, "PATCH", "/api/users/1", strings.NewReader(`{"email":"nope"}`)), 422)
	if !detailNamed(p.Details, "EmailValid", "email") {
		t.Fatalf("update validation: details = %+v, want EmailValid on email", p.Details)
	}
	if rec := call(h, "GET", app.PathAPIUserGet(1), nil); !strings.Contains(rec.Body.String(), "one@example.com") {
		t.Fatalf("a refused update reached the row: %s", rec.Body.String())
	}
	p = problemOf(t, call(h, "POST", "/api/users", strings.NewReader(`{"email":"one@example.com"}`)), 409)
	if len(p.Details) != 1 || !detailNamed(p.Details, "unique", "email") {
		t.Fatalf("duplicate: details = %+v, want one unique detail on email", p.Details)
	}
}

// TestQueryRouteValidationAttribution (§formats, §changesets): the error
// body is a Problem whose details name every failing check and its
// columns, so the GUI can mark the fields; a key that names no column
// is an unknown_field detail of the same 422, never dropped.
func TestQueryRouteValidationAttribution(t *testing.T) {
	h := handlerWithDB(t)
	p := problemOf(t, call(h, "POST", "/api/users", strings.NewReader(`{"email":""}`)), 422)
	for _, d := range p.Details {
		if len(d.Columns) != 1 || d.Columns[0] != "email" {
			t.Errorf("detail %+v should name the email column", d)
		}
	}
	for _, check := range []string{"required", "EmailShape", "EmailValid"} {
		if !detailNamed(p.Details, check, "email") {
			t.Errorf("details = %+v, want %s on email", p.Details, check)
		}
	}
	p = problemOf(t, call(h, "POST", "/api/users", strings.NewReader(`{"emial":"x@y.example","email":"x@y.example"}`)), 422)
	if len(p.Details) != 1 || !detailNamed(p.Details, "unknown_field", "emial") {
		t.Fatalf("unknown field: details = %+v, want one unknown_field detail on emial", p.Details)
	}
	p = problemOf(t, call(h, "PATCH", "/api/users/1", strings.NewReader(`{"emial":"x@y.example"}`)), 422)
	if !detailNamed(p.Details, "unknown_field", "emial") {
		t.Fatalf("unknown field on update: details = %+v, want unknown_field on emial", p.Details)
	}
	rec := call(h, "POST", "/api/users", strings.NewReader(`{"id":7,"email":"seven@example.com"}`))
	var made db.User
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &made) != nil || made.ID == 7 || made.Email != "seven@example.com" {
		t.Fatalf("a column outside the allow list is ignored, never an error (§changesets): %d %s", rec.Code, rec.Body.String())
	}
}
