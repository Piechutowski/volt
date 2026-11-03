package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/scopes/app"
	"github.com/Piechutowski/volt/examples/scopes/app/client"
)

// TestNearestErrorHandlerAnswers calls every route the window offers:
// the two that render answer 200, and each failure is answered by the
// error handler of its own scope.
func TestNearestErrorHandlerAnswers(t *testing.T) {
	base, err := serve()
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(base)
	want := map[string]struct {
		status  int
		handler string
	}{
		"/hello": {200, ""}, "/broken": {500, "Errors"},
		"/admin/stats": {200, ""}, "/admin/broken": {500, "AdminErrors"},
	}
	for _, rt := range routes {
		resp, err := rt.call(c)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		w := want[rt.path]
		if resp.StatusCode != w.status || resp.Header.Get("X-Error-Handler") != w.handler {
			t.Errorf("%s = %d by %q, want %d by %q", rt.path, resp.StatusCode, resp.Header.Get("X-Error-Handler"), w.status, w.handler)
		}
	}
	if !strings.Contains(replyText("/admin/broken", &http.Response{Status: "500", Header: http.Header{"X-Error-Handler": {"AdminErrors"}}, Body: io.NopCloser(strings.NewReader("{}"))}, nil), "answered by AdminErrors") {
		t.Error("the reply text does not name the error handler")
	}
	r := volt.RequestOf(httptest.NewRequest("GET", "/hello", nil), "GET /hello", time.Unix(0, 0), nil)
	if differ := app.ControllersDeterministic(app.Controllers{Pages: app.Pages{}, Admin: app.Admin{}}, r); len(differ) != 0 {
		t.Errorf("handlers that answered two values for one request: %v", differ)
	}
}
