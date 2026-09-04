// Query routes (spec §V4.8, §V4.9): the generated handlers bind
// parameters from path, body and query string, call the data package's
// queries against a real SQLite, and render in the negotiated format.
// Every status the spec names is served here, in both formats.
package itest

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/db"
)

// call issues one request with optional headers and body.
func call(h http.Handler, method, target string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestQueryRouteCRUDInJSON(t *testing.T) {
	h := handlerWithDB(t)

	// Create: body decoded by Content-Type (absent = JSON), 201 with the row.
	rec := call(h, "POST", "/api/users", strings.NewReader(`{"email":"two@example.com"}`))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var made db.User
	if err := json.Unmarshal(rec.Body.Bytes(), &made); err != nil || made.Email != "two@example.com" || made.ID == 0 {
		t.Fatalf("create body %q (%v)", rec.Body.String(), err)
	}

	// List: 200, both rows.
	rec = call(h, "GET", app.PathAPIUserList(), nil)
	var all []db.User
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &all) != nil || len(all) != 2 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	// Get: path parameter typed int32; a miss is the spine's 404.
	rec = call(h, "GET", app.PathAPIUserGet(made.ID), nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "two@example.com") {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call(h, "GET", app.PathAPIUserGet(999), nil); rec.Code != 404 {
		t.Fatalf("get miss: %d, want 404", rec.Code)
	}
	if rec = call(h, "GET", "/api/users/abc", nil); rec.Code != 404 {
		t.Fatalf("get with a non-int32 id: %d, want 404 (§V4.1.3)", rec.Code)
	}

	// Update: PATCH with a body, 200 with the rewritten row.
	rec = call(h, "PATCH", "/api/users/1", strings.NewReader(`{"email":"uno@example.com"}`), "Content-Type", volt.MIMEJSON)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "uno@example.com") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if rec = call(h, "PATCH", "/api/users/999", strings.NewReader(`{"email":"x@example.com"}`)); rec.Code != 404 {
		t.Fatalf("update miss: %d, want 404", rec.Code)
	}

	// Delete: 204 and no body; a second delete misses.
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
	// Absent key: the empty list, matching nothing — never an error.
	rec = call(h, "GET", app.PathAPIUserPicked(), nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("picked with no ids: %d %q, want 200 []", rec.Code, rec.Body.String())
	}
	// An element that does not parse names the parameter in a 400.
	rec = call(h, "GET", "/api/picked?ids=1&ids=x", nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "ids") {
		t.Fatalf("bad list element: %d %q", rec.Code, rec.Body.String())
	}
}

func TestQueryRouteGOBBothWays(t *testing.T) {
	h := handlerWithDB(t)

	// A GOB body creates; a GOB Accept reads rows back into the same type.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(db.UserCreateParams{Email: "gob@example.com"}); err != nil {
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
}

// TestQueryRouteTable: query rows carry the reference as written and no
// controller, and only reads have helpers (§V4.8).
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
	if seen != 8 {
		t.Errorf("query rows = %d, want 8 (six query routes, two dataset members)", seen)
	}
}

// TestDatasetRoutesServe: the expanded routes serve the member's select
// with the select's parameters bound from the query string (§V13).
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

// TestQueryRouteValidation (§V12.6, §V12.7, §V4.8.3): a violated check
// is a 422 naming the check before the database sees the row; a
// duplicate key is a 409 naming the column; both in either format.
func TestQueryRouteValidation(t *testing.T) {
	h := handlerWithDB(t)

	rec := call(h, "POST", "/api/users", strings.NewReader(`{"email":"nope"}`))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "EmailValid") {
		t.Fatalf("Go-reference check: %d %q, want 422 naming the check", rec.Code, rec.Body.String())
	}
	rec = call(h, "POST", "/api/users", strings.NewReader(`{"email":"@"}`))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "email_shape") {
		t.Fatalf("typed check: %d %q, want 422 naming email_shape", rec.Code, rec.Body.String())
	}
	rec = call(h, "PATCH", "/api/users/1", strings.NewReader(`{"email":"nope"}`))
	if rec.Code != 422 {
		t.Fatalf("update validation: %d, want 422", rec.Code)
	}
	// The seeded user is one@example.com: a duplicate is the database's
	// refusal, translated to 409 naming the column.
	rec = call(h, "POST", "/api/users", strings.NewReader(`{"email":"one@example.com"}`))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "users.email") {
		t.Fatalf("duplicate: %d %q, want 409 naming users.email", rec.Code, rec.Body.String())
	}
}

// TestQueryRouteValidationAttribution (§V4.9.5, §V12.7): the error body
// is a Problem whose details name the failing check and its columns,
// so the GUI can mark the field.
func TestQueryRouteValidationAttribution(t *testing.T) {
	h := handlerWithDB(t)
	rec := call(h, "POST", "/api/users", strings.NewReader(`{"email":""}`))
	var p volt.Problem
	if rec.Code != 422 || json.Unmarshal(rec.Body.Bytes(), &p) != nil {
		t.Fatalf("empty email: %d %q", rec.Code, rec.Body.String())
	}
	names := map[string]bool{}
	for _, d := range p.Details {
		if len(d.Columns) != 1 || d.Columns[0] != "email" {
			t.Errorf("detail %+v should name the email column", d)
		}
		names[d.Check] = true
	}
	if !names["email_required"] || !names["email_shape"] {
		t.Errorf("details = %+v, want email_required and email_shape", p.Details)
	}
	// A typo'd field is a 400 naming it, never silently dropped.
	rec = call(h, "POST", "/api/users", strings.NewReader(`{"emial":"x@y"}`))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "emial") {
		t.Fatalf("unknown field: %d %q", rec.Code, rec.Body.String())
	}
	// A duplicate attributes to the column too.
	rec = call(h, "POST", "/api/users", strings.NewReader(`{"email":"one@example.com"}`))
	p = volt.Problem{}
	if rec.Code != 409 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || len(p.Details) != 1 || p.Details[0].Columns[0] != "email" {
		t.Fatalf("duplicate: %d %q", rec.Code, rec.Body.String())
	}
}
