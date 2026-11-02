// Package app is the itest fixture: the hand-written side of the
// generated router — middleware and the error handler the routes.volt
// file references by name.
package app

import (
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/itest/blog/db"
)

// TagOuter and TagInner record middleware execution order in the
// X-Order response header; the proof suite asserts declaration order.
func TagOuter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "outer")
		next.ServeHTTP(w, r)
	})
}

func TagInner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "inner")
		next.ServeHTTP(w, r)
	})
}

// Errors is the error_handler declared in routes.volt: it marks the
// response so the proof suite can tell the DSL-configured spine ran,
// over the default mapping.
func Errors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "app")
}

// OpsErrors is the /ops scope's own error_handler: nearest wins
// (§scopes), so routes under /ops reach this one, not Errors.
func OpsErrors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "ops")
}

// TagExtra marks the nested scope's appended pipeline (§scopes).
func TagExtra(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "extra")
		next.ServeHTTP(w, r)
	})
}

// LoginForm is the struct body of the login route (§request-bodies):
// the shim decodes the request into it before the handler runs.
type LoginForm struct {
	Email    string
	Password string
}

// CurrentUser is the scope load of /me (§scope-loads): the user the
// X-User header names, or a 401 that halts the request; a session is
// not part of the language yet, so a header stands in for it.
func CurrentUser(r volt.Request) volt.Load[db.User] {
	id, err := strconv.Atoi(r.Header().Get("X-User"))
	if err != nil {
		return volt.Halt[db.User](volt.Reject(http.StatusUnauthorized, "sign in"))
	}
	return volt.LoadWith(db.UserGet(id))
}
