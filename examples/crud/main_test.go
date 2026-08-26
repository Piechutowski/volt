package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/examples/crud/app"
)

// do issues one request against the example's handler.
func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// TestGeneratedCRUD walks one resource through its whole life cycle
// over the generated router: create, read, list, update (both verbs),
// delete, and the 404 after.
func TestGeneratedCRUD(t *testing.T) {
	h := newHandler()

	rec := do(t, h, "POST", "/posts", `{"title":"first","body":"hello"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, want 201", rec.Code)
	}
	var created app.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: bad body %q", rec.Body.String())
	}

	// The generated helper builds the URL; no path string is spelled twice.
	url := app.PathPost(created.ID)
	if url != "/posts/1" {
		t.Fatalf("PathPost = %q, want /posts/1", url)
	}

	if rec := do(t, h, "GET", url, ""); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), "first") {
		t.Errorf("show: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "GET", app.PathPosts(), ""); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), "first") {
		t.Errorf("index: %d %q", rec.Code, rec.Body.String())
	}

	// update is registered for PATCH and PUT, one method behind both.
	for _, method := range []string{"PATCH", "PUT"} {
		rec := do(t, h, method, url, `{"title":"`+method+`"}`)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), method) {
			t.Errorf("%s: %d %q", method, rec.Code, rec.Body.String())
		}
	}

	if rec := do(t, h, "DELETE", url, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status %d, want 204", rec.Code)
	}
	if rec := do(t, h, "GET", url, ""); rec.Code != 404 {
		t.Errorf("show after delete: status %d, want 404", rec.Code)
	}
}

// TestAPISubset proves [api] and except: really removed routes: the
// form actions and DELETE are not registered at all.
func TestAPISubset(t *testing.T) {
	h := newHandler()

	if rec := do(t, h, "POST", "/api/comments", `{"post_id":1,"author":"me","body":"nice"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create comment: status %d", rec.Code)
	}
	if rec := do(t, h, "GET", app.PathAPIComment(1), ""); rec.Code != 200 {
		t.Errorf("show comment: status %d", rec.Code)
	}

	// [api] drops new/edit …
	if rec := do(t, h, "GET", "/api/comments/new", ""); rec.Code == 200 {
		t.Error("/api/comments/new should not exist under [api]")
	}
	// … and except:(delete) drops DELETE — 405, since the path exists
	// for other methods.
	if rec := do(t, h, "DELETE", "/api/comments/1", ""); rec.Code != 405 {
		t.Errorf("DELETE comment: status %d, want 405", rec.Code)
	}

	// The HTML resource keeps its form actions.
	if rec := do(t, h, "GET", app.PathNewPost(), ""); rec.Code != 200 {
		t.Errorf("/posts/new: status %d", rec.Code)
	}
}

// TestTypedParam404 proves §V4.1.3: the id is typed, so a non-numeric
// id is that route's 404 — the handler never runs.
func TestTypedParam404(t *testing.T) {
	h := newHandler()
	for _, target := range []string{"/posts/abc", "/posts/99999999999"} {
		if rec := do(t, h, "GET", target, ""); rec.Code != 404 {
			t.Errorf("GET %s: status %d, want 404", target, rec.Code)
		}
	}
}

// TestRouteTable: the generated table is the route list as data.
func TestRouteTable(t *testing.T) {
	if len(app.Table) != 14 {
		t.Fatalf("route table has %d rows, want 14", len(app.Table))
	}
}
