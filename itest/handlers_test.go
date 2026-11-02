package itest

import (
	"net/http"
	"net/http/httptest"
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
	if !volt.Same(users{}.Profile(r, 7), volt.Then(db.UserGet(7), func(db.User) volt.Response { return volt.NoContent() })) {
		t.Error("Profile does not describe the lookup of its user")
	}
	if volt.Same(users{}.Profile(r, 7), users{}.Profile(r, 8)) {
		t.Error("Profile of two users describes one effect")
	}
	c := app.Controllers{Home: home{}, Users: users{}, Files: files{}, Admin: admin{}, Ops: ops{}, Tags: tags{}, Pages: pages{}, Archive: archive{}}
	if differ := app.ControllersDeterministic(c, r); len(differ) != 0 {
		t.Errorf("handlers that answered two different values for one request: %v", differ)
	}
}

// TestEffectFailureInsideThenReachesTheSpine proves an effect that
// fails inside a continuation goes to the route's error handler (spec
// §requests-and-responses): the lookup of a missing user is a 404
// through the scope's own error_handler.
func TestEffectFailureInsideThenReachesTheSpine(t *testing.T) {
	h := handler()
	rec, body := serve(t, h, "GET", app.PathProfile(1))
	if rec.Code != 200 || body["email"] != "one@example.com" {
		t.Fatalf("profile of user 1 = %d %v", rec.Code, body)
	}
	rec, _ = serve(t, h, "GET", app.PathProfile(999))
	if rec.Code != http.StatusNotFound || rec.Header().Get("X-Error-Handler") != "app" {
		t.Fatalf("profile of a missing user = %d, error handler %q; want 404 through the scope's handler", rec.Code, rec.Header().Get("X-Error-Handler"))
	}
}
