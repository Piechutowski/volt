// Query routes (spec §query-routes, §formats): the generated handlers bind
// parameters from path, body and query string, call the data package's
// queries against a float64 SQLite, and render in the negotiated format.
// Every status the spec names is served here, in both formats.
package itest

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

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
	if seen != 14 {
		t.Errorf("query rows = %d, want 14 (six query routes, two dataset members, six resources [default] routes)", seen)
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
