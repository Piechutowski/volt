package itest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/app"
	"github.com/Piechutowski/volt/itest/blog/db"
)

// TestHandlersAreValues proves a handler is a function from a request
// value to a response value (spec §requests-and-responses): called
// twice with one request it answers the same value, the generated
// determinism helper finds no handler that differs, and a handler that
// reaches the database returns the continuation, so it is tested with
// no database at all.
func TestHandlersAreValues(t *testing.T) {
	r := volt.RequestOf(httptest.NewRequest("GET", "/users/7/profile", nil), "GET /users/{id}/profile", time.Unix(1700000000, 0), nil, "id")
	if !volt.Same(users{}.Show(r, 7), users{}.Show(r, 7)) || volt.Same(users{}.Show(r, 7), users{}.Show(r, 8)) {
		t.Error("Show is not a function of its inputs")
	}
	if !volt.Same(users{}.Card(r, 7), volt.Then(db.UserGet(7), func(db.User) volt.Response { return volt.NoContent() })) {
		t.Error("Card does not describe the lookup of its user")
	}
	if volt.Same(users{}.Card(r, 7), users{}.Card(r, 8)) {
		t.Error("Card of two users describes one effect")
	}
	if !volt.Same(users{}.Profile(r, 7, db.User{Email: "a"}), volt.Render(db.User{Email: "a"})) {
		t.Error("Profile does not render the user it was handed")
	}
	c := app.Controllers{Home: home{}, Users: users{}, Files: files{}, Admin: admin{}, Ops: ops{}, Tags: tags{}, Pages: pages{}, Archive: archive{}, Session: session{}}
	if differ := app.ControllersDeterministic(c, r); len(differ) != 0 {
		t.Errorf("handlers that answered two different values for one request: %v", differ)
	}
}

// TestEffectFailureInsideThenReachesTheSpine proves an effect that
// fails inside a continuation, and a load that fails before the
// handler, go to the route's error handler (spec
// §requests-and-responses, §route-loads): the lookup of a missing user
// is a 404 through the scope's own error_handler either way.
func TestEffectFailureInsideThenReachesTheSpine(t *testing.T) {
	h := handler()
	for _, path := range []string{app.PathCard(1), app.PathProfile(1)} {
		rec, body := serve(t, h, "GET", path)
		if rec.Code != 200 || body["email"] != "one@example.com" {
			t.Fatalf("%s = %d %v", path, rec.Code, body)
		}
	}
	for _, path := range []string{app.PathCard(999), app.PathProfile(999)} {
		rec, _ := serve(t, h, "GET", path)
		if rec.Code != http.StatusNotFound || rec.Header().Get("X-Error-Handler") != "app" {
			t.Fatalf("%s = %d, error handler %q; want 404 through the scope's handler", path, rec.Code, rec.Header().Get("X-Error-Handler"))
		}
	}
}

// TestRequestBodies proves spec §request-bodies end to end: a
// changeset body is decoded, loaded, cast with the narrowed allow list
// and validated before the handler, which sees the findings and
// decides; a struct body decodes with volt.Decode's rules; a body that
// does not decode is a 400 before anything loads, and a missing row of
// an update is a 404 through the scope's error handler.
func TestRequestBodies(t *testing.T) {
	h := handler()
	post := func(method, path, contentType, body string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("%s %s: bad JSON %q", method, path, rec.Body.String())
			}
		}
		return rec, out
	}

	rec, body := post("PATCH", app.PathChangeEmail(1), "application/json", `{"email": "two@example.com"}`)
	if rec.Code != 200 || body["email"] != "two@example.com" {
		t.Fatalf("valid change = %d %v; want 200 with the stored row", rec.Code, body)
	}
	rec, body = post("PATCH", app.PathChangeEmail(1), "application/json", `{"email": "not-an-email"}`)
	if rec.Code != http.StatusUnprocessableEntity || body["message"] != "check the fields" {
		t.Fatalf("invalid change = %d %v; want the handler's own 422", rec.Code, body)
	}
	if details, _ := body["details"].([]any); len(details) == 0 {
		t.Errorf("the 422 carries no details; want the changeset's findings")
	}
	rec, body = post("PATCH", app.PathChangeEmail(1), "application/json", `{"created_at": "2001-01-01T00:00:00Z"}`)
	if rec.Code != 200 || strings.HasPrefix(body["created_at"].(string), "2001") {
		t.Fatalf("a column outside only: = %d %v; want 200 with the column left alone", rec.Code, body)
	}
	rec, body = post("PATCH", app.PathChangeEmail(1), "application/json", `{"colour": "red"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a key that is no column = %d %v; want 422, the cast's finding", rec.Code, body)
	}
	rec, _ = post("PATCH", app.PathChangeEmail(1), "application/json", `{"email"`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body = %d; want 400", rec.Code)
	}
	rec, _ = post("PATCH", app.PathChangeEmail(1), "text/plain", `email=x`)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text body = %d; want 415", rec.Code)
	}
	rec, _ = post("PATCH", app.PathChangeEmail(999), "application/json", `{"email": "two@example.com"}`)
	if rec.Code != http.StatusNotFound || rec.Header().Get("X-Error-Handler") != "app" {
		t.Errorf("missing row = %d via %q; want 404 through the scope's handler", rec.Code, rec.Header().Get("X-Error-Handler"))
	}

	rec, body = post("POST", app.PathLogin(), "application/json", `{"Email": "one@example.com", "Password": "open sesame"}`)
	if rec.Code != 200 || body["email"] != "one@example.com" {
		t.Fatalf("login = %d %v; want 200", rec.Code, body)
	}
	rec, _ = post("POST", app.PathLogin(), "application/json", `{"Email": "one@example.com", "Password": "nope"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password = %d; want the handler's 401", rec.Code)
	}
	rec, body = post("POST", app.PathLogin(), "application/json", `{"Email": "one@example.com", "Remember": true}`)
	if rec.Code != http.StatusBadRequest || body["message"] != `unknown field "Remember"` {
		t.Errorf("unknown field = %d %v; want 400 naming it", rec.Code, body)
	}

	r := volt.RequestOf(httptest.NewRequest("POST", "/login", nil), "POST /login", time.Unix(1700000000, 0), nil)
	if !volt.Same(session{}.Login(r, app.LoginForm{Password: "open sesame"}), session{}.Login(r, app.LoginForm{Password: "open sesame"})) {
		t.Error("Login is not a function of its body")
	}
}
