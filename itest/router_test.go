// Package itest proves the generated router's runtime semantics
// (docs/spec.md, "Conformance and the proof chain", link 4): the committed
// fixture under blog/ is served over httptest and its behavior asserted
// against the spec — matching, typed-parameter 404s, pipeline order,
// the error spine, and reverse-URL round-trip totality.
package itest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/db"
	"github.com/Piechutowski/volt/nao/rt"

	_ "github.com/mattn/go-sqlite3"
)

// echo answers every route with its matched pattern and parameters, so
// each assertion can see exactly which route dispatched.
func echo(w http.ResponseWriter, r *volt.Request, params ...any) error {
	return volt.JSON(w, map[string]any{"route": r.Route(), "params": fmt.Sprint(params...)})
}

type home struct{}

func (home) Index(w http.ResponseWriter, r *volt.Request) error { return echo(w, r) }
func (home) Ping(w http.ResponseWriter, r *volt.Request) error  { return echo(w, r) }
func (home) Teapot(w http.ResponseWriter, r *volt.Request) error {
	return volt.Error(http.StatusTeapot, "short and stout")
}

type users struct{}

func (users) Index(w http.ResponseWriter, r *volt.Request) error  { return echo(w, r) }
func (users) New(w http.ResponseWriter, r *volt.Request) error    { return echo(w, r) }
func (users) Create(w http.ResponseWriter, r *volt.Request) error { return echo(w, r) }
func (users) Show(w http.ResponseWriter, r *volt.Request, id int32) error {
	return echo(w, r, id)
}
func (users) Edit(w http.ResponseWriter, r *volt.Request, id int32) error {
	return echo(w, r, id)
}
func (users) Update(w http.ResponseWriter, r *volt.Request, id int32) error {
	return echo(w, r, id)
}
func (users) Delete(w http.ResponseWriter, r *volt.Request, id int32) error {
	return echo(w, r, id)
}
func (users) Avatar(w http.ResponseWriter, r *volt.Request, id int32) error {
	return echo(w, r, id)
}

type files struct{}

func (files) Serve(w http.ResponseWriter, r *volt.Request, path string) error {
	return echo(w, r, path)
}

type admin struct{}

func (admin) Stats(w http.ResponseWriter, r *volt.Request) error { return echo(w, r) }

type ops struct{}

func (ops) Fail(w http.ResponseWriter, r *volt.Request) error {
	return volt.Error(http.StatusServiceUnavailable, "ops down")
}

// tags, pages and archive exercise the three explicit parameter types:
// string, int and int64 (§V4.1.3).
type tags struct{}

func (tags) Show(w http.ResponseWriter, r *volt.Request, name string) error {
	return echo(w, r, name)
}

type pages struct{}

func (pages) Show(w http.ResponseWriter, r *volt.Request, num int) error {
	return echo(w, r, num)
}

type archive struct{}

func (archive) Show(w http.ResponseWriter, r *volt.Request, stamp int64) error {
	return echo(w, r, stamp)
}

func handler() http.Handler {
	return handlerWithDB(nil)
}

// handlerWithDB wires the fixture router over a real database (query
// routes, §V4.8) — an in-memory SQLite loaded with the generated DDL —
// seeded with one user so reads by id 1 succeed.
func handlerWithDB(t *testing.T) http.Handler {
	h, _ := fixture(t)
	return h
}

// fixture is handlerWithDB plus the event broker the router serves
// (§V4.11), so a test can publish and watch the stream.
func fixture(t *testing.T) (http.Handler, *volt.Broker) {
	if t == nil {
		t = &testing.T{}
	}
	sqlDB, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	ddl, err := os.ReadFile(filepath.Join("blog", "db", "nao_schema.sql"))
	if err != nil {
		panic(err)
	}
	if _, err := sqlDB.Exec(string(ddl)); err != nil {
		panic(err)
	}
	q := db.New(sqlDB)
	if _, err := q.UserCreate(context.Background(), db.UserCreateParams{Email: "one@example.com"}); err != nil {
		panic(err)
	}
	broker := &volt.Broker{Heartbeat: 20 * time.Millisecond}
	return app.New(app.Controllers{
		Home: home{}, Users: users{}, Files: files{}, Admin: admin{},
		Ops: ops{}, Tags: tags{}, Pages: pages{}, Archive: archive{},
		DB:     q,
		Events: broker,
	}), broker
}

// serve issues one request and decodes the echo payload when present.
func serve(t *testing.T, h http.Handler, method, target string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	var body map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		var v any
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatalf("bad JSON body %q: %v", rec.Body.String(), err)
		}
		body, _ = v.(map[string]any) // query routes answer rows, not the echo object
	}
	return rec, body
}

// TestRoundTripTotality proves §V4.6 by enumeration, not sampling: for
// EVERY route in the generated table that has a helper, the URL built
// by that helper dispatches back to that same route. The count
// assertion makes the enumeration total — a new named route without a
// round-trip entry fails the test.
func TestRoundTripTotality(t *testing.T) {
	h := handler()

	// One call per generated Path* helper, keyed by helper name.
	built := map[string]struct {
		method string
		url    string
	}{
		"PathRoot":       {"GET", app.PathRoot()},
		"PathTeapot":     {"GET", app.PathTeapot()},
		"PathPing":       {"GET", app.PathPing()},
		"PathAdminStats": {"GET", app.PathAdminStats()},
		"PathUsers":      {"GET", app.PathUsers()},
		"PathNewUser":    {"GET", app.PathNewUser()},
		"PathUser":       {"GET", app.PathUser(7)},
		"PathEditUser":   {"GET", app.PathEditUser(7)},
		"PathAvatar":     {"GET", app.PathAvatar(7)},
		"PathServe":      {"GET", app.PathServe("a/b c.txt")},
		"PathOpsFail":    {"GET", app.PathOpsFail()},
		"PathTag":        {"GET", app.PathTag("hello world")},
		"PathPage":       {"GET", app.PathPage(3)},
		"PathArchive":    {"GET", app.PathArchive(1755000000123)},
		// Query routes (§V4.8): the seeded user is id 1.
		"PathAPIUserList":   {"GET", app.PathAPIUserList()},
		"PathAPIUserGet":    {"GET", app.PathAPIUserGet(1)},
		"PathAPIUserPicked": {"GET", app.PathAPIUserPicked(volt.Query("ids", "1"))},
		// Dataset routes (§V13): one per member of the group select.
		"PathMsRevenueBrowse": {"GET", app.PathMsRevenueBrowse(volt.Query("year", "2024"))},
		"PathMsUsageBrowse":   {"GET", app.PathMsUsageBrowse(volt.Query("year", "2024"))},
		"PathRefUsers":        {"GET", app.PathRefUsers()},
		"PathRefUser":         {"GET", app.PathRefUser(1)},
		// The event route (§V4.11) streams forever; its round trip is
		// TestEventRouteStreams, so the helper is listed and not served.
		"PathEvents": {"GET", app.PathEvents()},
	}
	queryRoutes := map[string]bool{} // helper -> served by a generated query handler
	for _, r := range app.Table {
		if r.Query != "" && r.Helper != "" {
			queryRoutes[r.Helper] = true
		}
	}

	tableHelpers := map[string]string{} // helper -> full registered pattern
	for _, r := range app.Table {
		if r.Helper == "" {
			continue
		}
		pattern := r.Pattern
		if r.Method != "" {
			pattern = r.Method + " " + r.Pattern
		}
		tableHelpers[r.Helper] = pattern
	}
	if len(tableHelpers) != len(built) {
		t.Fatalf("route table has %d helpers, round-trip covers %d — a helper is missing from this test: table=%v",
			len(tableHelpers), len(built), tableHelpers)
	}

	for helper, want := range tableHelpers {
		b, ok := built[helper]
		if !ok {
			t.Errorf("no round-trip entry for %s", helper)
			continue
		}
		if helper == "PathEvents" {
			continue // a stream never returns to a recorder
		}
		rec, body := serve(t, h, b.method, b.url)
		if helper == "PathTeapot" {
			// dispatches, then errors by design; the spine test covers it
			if rec.Code != http.StatusTeapot {
				t.Errorf("%s: status %d", helper, rec.Code)
			}
			continue
		}
		if helper == "PathOpsFail" {
			// errors by design; the nearest-wins test covers it
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s: status %d", helper, rec.Code)
			}
			continue
		}
		if rec.Code != 200 {
			t.Errorf("%s → %s: status %d, want 200", helper, b.url, rec.Code)
			continue
		}
		if queryRoutes[helper] {
			// A query route renders rows, not the echo; a 200 with a JSON
			// body from the seeded database is its dispatch proof.
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Errorf("%s → %s: query route answered %q, want JSON", helper, b.url, rec.Header().Get("Content-Type"))
			}
			continue
		}
		if got := body["route"]; got != want {
			t.Errorf("%s → %s dispatched to %v, want %s", helper, b.url, got, want)
		}
	}
}

func TestTypedParamParseFailureIs404(t *testing.T) {
	h := handler()
	// §V4.1.3: a non-int32 id is this route's 404, not a fallthrough.
	for _, target := range []string{
		"/users/abc", "/users/abc/avatar", "/users/99999999999",
		"/pages/abc", "/pages/99999999999999999999", // int
		"/archive/1.5", "/archive/99999999999999999999", // int64
	} {
		rec, _ := serve(t, h, "GET", target)
		if rec.Code != 404 {
			t.Errorf("GET %s: status %d, want 404", target, rec.Code)
		}
	}
	// The valid spellings still work.
	if rec, _ := serve(t, h, "GET", "/users/42"); rec.Code != 200 {
		t.Errorf("GET /users/42: status %d", rec.Code)
	}
}

func TestUnknownPathIs404AndWrongMethodIs405(t *testing.T) {
	h := handler()
	if rec, _ := serve(t, h, "GET", "/nope"); rec.Code != 404 {
		t.Errorf("GET /nope: %d, want 404", rec.Code)
	}
	// §V4.2.3: the root registers as /{$}, so an arbitrary path does
	// not fall through to it.
	if rec, _ := serve(t, h, "GET", "/nope/deeper"); rec.Code != 404 {
		t.Errorf("GET /nope/deeper: %d, want 404", rec.Code)
	}
	rec, _ := serve(t, h, "DELETE", "/teapot")
	if rec.Code != 405 {
		t.Errorf("DELETE /teapot: %d, want 405 (ServeMux synthesizes it)", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("405 Allow header = %q, want it to include GET", allow)
	}
}

func TestAnyVerbMatchesEveryMethod(t *testing.T) {
	h := handler()
	for _, m := range []string{"GET", "POST", "DELETE", "PATCH"} {
		rec, body := serve(t, h, m, "/ping")
		if rec.Code != 200 || body["route"] != "/ping" {
			t.Errorf("%s /ping: status %d route %v", m, rec.Code, body["route"])
		}
	}
}

func TestResourcesUpdateSpansPatchAndPut(t *testing.T) {
	h := handler()
	for _, m := range []string{"PATCH", "PUT"} {
		rec, body := serve(t, h, m, "/users/7")
		if rec.Code != 200 || body["route"] != m+" /users/{id}" {
			t.Errorf("%s /users/7: status %d route %v", m, rec.Code, body["route"])
		}
	}
	if rec, body := serve(t, h, "POST", "/users"); rec.Code != 200 || body["route"] != "POST /users" {
		t.Errorf("POST /users: %d %v", rec.Code, body["route"])
	}
}

func TestWildcardCapturesRest(t *testing.T) {
	h := handler()
	rec, body := serve(t, h, "GET", "/files/a/b%20c.txt")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if body["params"] != "a/b c.txt" {
		t.Errorf("wildcard params = %v, want 'a/b c.txt'", body["params"])
	}
}

// TestPipelineOrder proves §V4.4.2: plugs run in declaration order,
// outermost first — volt.RequestID, TagOuter, TagInner.
func TestPipelineOrder(t *testing.T) {
	h := handler()
	rec, _ := serve(t, h, "GET", "/")
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("volt.RequestID did not run")
	}
	if got := strings.Join(rec.Header().Values("X-Order"), ","); got != "outer,inner" {
		t.Errorf("plug order = %q, want outer,inner (declaration order)", got)
	}
}

// TestNestedScopePipelinesAppend proves §V4.4.1: a nested scope's pipe
// appends after the ancestors' — api's plugs run before extra's.
func TestNestedScopePipelinesAppend(t *testing.T) {
	h := handler()
	rec, body := serve(t, h, "GET", app.PathAdminStats())
	if rec.Code != 200 || body["route"] != "GET /admin/stats" {
		t.Fatalf("dispatch wrong: %d %v", rec.Code, body["route"])
	}
	if got := strings.Join(rec.Header().Values("X-Order"), ","); got != "outer,inner,extra" {
		t.Errorf("appended pipe order = %q, want outer,inner,extra", got)
	}
}

// TestHeadMatchesGet documents the inherited ServeMux rule (§V4.2.3):
// a GET registration also serves HEAD.
func TestHeadMatchesGet(t *testing.T) {
	h := handler()
	rec, _ := serve(t, h, "HEAD", "/about-nothing-here")
	if rec.Code != 404 {
		t.Errorf("HEAD unknown: %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest("HEAD", "/users", nil))
	if rec2.Code != 200 {
		t.Errorf("HEAD on a GET route: %d, want 200 (ServeMux GET-matches-HEAD)", rec2.Code)
	}
}

// TestQueryOptionsEndToEnd: a helper-built URL with query options still
// dispatches to its route, params intact.
func TestQueryOptionsEndToEnd(t *testing.T) {
	h := handler()
	url := app.PathUser(7, volt.Query("tab", "posts"), volt.Query("q", "a b"))
	if !strings.HasPrefix(url, "/users/7?") {
		t.Fatalf("built URL = %q", url)
	}
	rec, body := serve(t, h, "GET", url)
	if rec.Code != 200 || body["route"] != "GET /users/{id}" || body["params"] != "7" {
		t.Errorf("query-optioned URL misdispatched: %d %v %v", rec.Code, body["route"], body["params"])
	}
}

// TestErrorSpine proves §V4.4.3: a returned error reaches the
// DSL-declared error_handler, which maps HTTPError to its status.
func TestErrorSpine(t *testing.T) {
	h := handler()
	rec, _ := serve(t, h, "GET", "/teapot")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status %d, want 418", rec.Code)
	}
	if rec.Header().Get("X-Error-Handler") != "app" {
		t.Error("the DSL-declared error_handler did not run (default used instead)")
	}
	if !strings.Contains(rec.Body.String(), "short and stout") {
		t.Errorf("HTTPError message lost: %q", rec.Body.String())
	}
}

// TestNearestErrorHandlerWins proves §V4.4.3's nearest-wins rule: /ops
// declares its own error_handler, so its routes reach OpsErrors while
// everything else still reaches the root scope's Errors.
func TestNearestErrorHandlerWins(t *testing.T) {
	h := handler()
	rec, _ := serve(t, h, "GET", app.PathOpsFail())
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("X-Error-Handler"); got != "ops" {
		t.Errorf("error handler = %q, want ops (nearest scope's handler)", got)
	}
}

// TestParamTypesEndToEnd: each explicit parameter type round-trips its
// value through helper, escaping, ServeMux and the typed shim.
func TestParamTypesEndToEnd(t *testing.T) {
	h := handler()
	cases := []struct{ url, wantRoute, wantParams string }{
		{app.PathTag("hello world"), "GET /tags/{name}", "hello world"},
		{app.PathPage(3), "GET /pages/{num}", "3"},
		{app.PathArchive(1755000000123), "GET /archive/{stamp}", "1755000000123"},
	}
	for _, tc := range cases {
		rec, body := serve(t, h, "GET", tc.url)
		if rec.Code != 200 || body["route"] != tc.wantRoute || body["params"] != tc.wantParams {
			t.Errorf("%s: %d route=%v params=%v, want %s %q", tc.url, rec.Code, body["route"], body["params"], tc.wantRoute, tc.wantParams)
		}
	}
}

// TestTableMatchesServedReality: every row of the generated table is
// registered — issuing a request shaped from the row's pattern with
// sample values dispatches to exactly that pattern. This covers rows
// without helpers (Create/Update/Delete) too.
func TestTableMatchesServedReality(t *testing.T) {
	h := handler()
	for _, row := range app.Table {
		url := row.Pattern
		for _, p := range row.Params {
			sample := "42"
			if p.Type == "string" {
				sample = "abc"
			}
			if p.Wild {
				url = strings.Replace(url, "{"+p.Name+"...}", sample, 1)
			} else {
				url = strings.Replace(url, "{"+p.Name+"}", sample, 1)
			}
		}
		url = strings.Replace(url, "/{$}", "/", 1)
		method := row.Method
		if method == "" {
			method = "GET"
		}
		want := row.Pattern
		if row.Method != "" {
			want = row.Method + " " + row.Pattern
		}
		if row.Controller == "volt" && row.Action == "Events" {
			continue // the stream never returns to a recorder; TestEventRouteStreams covers it
		}
		rec, body := serve(t, h, method, url)
		if row.Spelled == "/teapot" || row.Spelled == "/ops/fail" {
			continue // error by design
		}
		if row.Query != "" {
			// A generated query handler owns the row: sample values may
			// miss (404 from the spine, never ServeMux's page) or lack a
			// body (400); what must not happen is a mux-level miss.
			if rec.Code == 405 || strings.HasPrefix(rec.Body.String(), "404 page not found") {
				t.Errorf("%s %s: not dispatched to the query route: %d %q", method, url, rec.Code, rec.Body.String())
			}
			continue
		}
		if rec.Code != 200 {
			t.Errorf("%s %s: status %d", method, url, rec.Code)
			continue
		}
		if body["route"] != want {
			t.Errorf("%s %s dispatched to %v, want %s", method, url, body["route"], want)
		}
	}
}
