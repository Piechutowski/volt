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
	"io"
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
func (users) Show(w http.ResponseWriter, r *volt.Request, id int) error {
	return echo(w, r, id)
}
func (users) Edit(w http.ResponseWriter, r *volt.Request, id int) error {
	return echo(w, r, id)
}
func (users) Update(w http.ResponseWriter, r *volt.Request, id int) error {
	return echo(w, r, id)
}
func (users) Delete(w http.ResponseWriter, r *volt.Request, id int) error {
	return echo(w, r, id)
}
func (users) Avatar(w http.ResponseWriter, r *volt.Request, id int) error {
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

type tags struct{}

func (tags) Show(w http.ResponseWriter, r *volt.Request, name string) error {
	return echo(w, r, name)
}

type pages struct{}

func (pages) Show(w http.ResponseWriter, r *volt.Request, num int) error {
	return echo(w, r, num)
}

type archive struct{}

func (archive) Show(w http.ResponseWriter, r *volt.Request, stamp int) error {
	return echo(w, r, stamp)
}

func handler() http.Handler {
	return handlerWithDB(nil)
}

func handlerWithDB(t *testing.T) http.Handler {
	h, _ := fixture(t)
	return h
}

func fixture(t *testing.T) (http.Handler, *volt.Broker) {
	if t == nil {
		t = &testing.T{}
	}
	sqlDB, err := rt.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	ddl, err := os.ReadFile(filepath.Join("blog", "db", "volt_schema.sql"))
	if err != nil {
		panic(err)
	}
	if _, err := sqlDB.Exec(string(ddl)); err != nil {
		panic(err)
	}
	q := db.New(sqlDB)
	if _, err := q.UserCreate(context.Background(), db.UserNew().EmailSet("one@example.com")); err != nil {
		panic(err)
	}
	if _, err := q.TagCreate(context.Background(), db.TagNew().SlugSet("go").NameSet("Go")); err != nil {
		panic(err)
	}
	broker := &volt.Broker{Heartbeat: 20 * time.Millisecond}
	return app.NewRouter(app.Controllers{
		Home: home{}, Users: users{}, Files: files{}, Admin: admin{},
		Ops: ops{}, Tags: tags{}, Pages: pages{}, Archive: archive{},
		DB:     q,
		Events: broker,
	}), broker
}

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
		body, _ = v.(map[string]any)
	}
	return rec, body
}

// TestRoundTripTotality proves §route-names-and-reverse-urls by enumeration, not sampling: for
// EVERY route in the generated table that has a helper, the URL built
// by that helper dispatches back to that same route. The count
// assertion makes the enumeration total — a new named route without a
// round-trip entry fails the test.
func TestRoundTripTotality(t *testing.T) {
	h := handler()

	built := map[string]struct {
		method string
		url    string
	}{
		"PathRoot":            {"GET", app.PathRoot()},
		"PathTeapot":          {"GET", app.PathTeapot()},
		"PathPing":            {"GET", app.PathPing()},
		"PathAdminStats":      {"GET", app.PathAdminStats()},
		"PathUsers":           {"GET", app.PathUsers()},
		"PathNewUser":         {"GET", app.PathNewUser()},
		"PathUser":            {"GET", app.PathUser(7)},
		"PathEditUser":        {"GET", app.PathEditUser(7)},
		"PathAvatar":          {"GET", app.PathAvatar(7)},
		"PathServe":           {"GET", app.PathServe("a/b c.txt")},
		"PathOpsFail":         {"GET", app.PathOpsFail()},
		"PathTag":             {"GET", app.PathTag("hello world")},
		"PathPage":            {"GET", app.PathPage(3)},
		"PathArchive":         {"GET", app.PathArchive(1755000000123)},
		"PathAPIUserList":     {"GET", app.PathAPIUserList()},
		"PathAPIUserGet":      {"GET", app.PathAPIUserGet(1)},
		"PathAPIUserPicked":   {"GET", app.PathAPIUserPicked(volt.Query("ids", "1"))},
		"PathAPIUserSince":    {"GET", app.PathAPIUserSince(volt.Query("since", "2000-01-01T00:00:00Z"))},
		"PathMsRevenueBrowse": {"GET", app.PathMsRevenueBrowse(volt.Query("year", "2024"))},
		"PathMsUsageBrowse":   {"GET", app.PathMsUsageBrowse(volt.Query("year", "2024"))},
		"PathRefUsers":        {"GET", app.PathRefUsers()},
		"PathRefUser":         {"GET", app.PathRefUser(1)},
		"PathRefTags":         {"GET", app.PathRefTags()},
		"PathRefTag":          {"GET", app.PathRefTag("go")},
		"PathEvents":          {"GET", app.PathEvents()},
	}
	queryRoutes := map[string]bool{}
	for _, r := range app.Table {
		if r.Query != "" && r.Helper != "" {
			queryRoutes[r.Helper] = true
		}
	}

	tableHelpers := map[string]string{}
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
			continue
		}
		rec, body := serve(t, h, b.method, b.url)
		if helper == "PathTeapot" {
			if rec.Code != http.StatusTeapot {
				t.Errorf("%s: status %d", helper, rec.Code)
			}
			continue
		}
		if helper == "PathOpsFail" {
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
	for _, target := range []string{
		"/users/abc", "/users/abc/avatar", "/users/99999999999999999999",
		"/pages/abc", "/pages/99999999999999999999",
		"/archive/1.5", "/archive/99999999999999999999",
	} {
		rec, _ := serve(t, h, "GET", target)
		if rec.Code != 404 {
			t.Errorf("GET %s: status %d, want 404", target, rec.Code)
		}
	}
	if rec, _ := serve(t, h, "GET", "/users/42"); rec.Code != 200 {
		t.Errorf("GET /users/42: status %d", rec.Code)
	}
}

func TestUnknownPathIs404AndWrongMethodIs405(t *testing.T) {
	h := handler()
	if rec, _ := serve(t, h, "GET", "/nope"); rec.Code != 404 {
		t.Errorf("GET /nope: %d, want 404", rec.Code)
	}
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

func TestResourcesUpdateIsPatch(t *testing.T) {
	h := handler()
	if rec, body := serve(t, h, "PATCH", "/users/7"); rec.Code != 200 || body["route"] != "PATCH /users/{id}" {
		t.Errorf("PATCH /users/7: status %d route %v", rec.Code, body["route"])
	}
	if rec, _ := serve(t, h, "PUT", "/users/7"); rec.Code != 405 || strings.Contains(rec.Header().Get("Allow"), "PUT") {
		t.Errorf("PUT /users/7: status %d, Allow %q; want 405 without PUT (§the-action-table)", rec.Code, rec.Header().Get("Allow"))
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

// TestPipelineOrder proves §scopes: plugs run in declaration order,
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

// TestNestedScopePipelinesAppend proves §scopes: a nested scope's pipe
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

// TestHeadMatchesGet documents the inherited ServeMux rule (§routes,
// §query-routes): a GET registration also serves HEAD, with the status
// and headers of the GET, generated query, default and dataset handlers included.
func TestHeadMatchesGet(t *testing.T) {
	h := handlerWithDB(t)
	for _, tc := range []struct {
		target string
		status int
	}{
		{"/users", 200},
		{"/api/users", 200},
		{"/api/users/1", 200},
		{"/api/picked?ids=1", 200},
		{"/ref/tags/go", 200},
		{"/ms/revenue?year=1", 200},
		{"/ref/tags/missing", 404},
		{"/api/users/999", 404},
		{"/ms/revenue", 400},
		{"/about-nothing-here", 404},
	} {
		get, head := call(h, "GET", tc.target, nil), call(h, "HEAD", tc.target, nil)
		if get.Code != tc.status || head.Code != tc.status {
			t.Errorf("%s: GET %d, HEAD %d, want %d for both", tc.target, get.Code, head.Code, tc.status)
		}
		if g, hd := get.Header().Get("Content-Type"), head.Header().Get("Content-Type"); g == "" || g != hd {
			t.Errorf("%s: Content-Type GET %q, HEAD %q, want the same", tc.target, g, hd)
		}
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Head(srv.URL + "/api/users/1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), volt.MIMEJSON) || len(body) != 0 {
		t.Errorf("HEAD over the wire: %d %q body %q (%v), want 200 JSON with no body", resp.StatusCode, resp.Header.Get("Content-Type"), body, err)
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

// TestErrorSpine proves §scopes: a returned error reaches the
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

// TestNearestErrorHandlerWins proves §scopes's nearest-wins rule: /ops
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
			continue
		}
		rec, body := serve(t, h, method, url)
		if row.Spelled == "/teapot" || row.Spelled == "/ops/fail" {
			continue
		}
		if row.Query != "" {
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
