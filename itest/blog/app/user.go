// Package app is the itest fixture: the hand-written side of the
// generated router — middleware and the error handler the routes.volt
// file references by name.
package app

import (
	"net/http"

	"github.com/Piechutowski/volt"
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
// then defers to the default mapping.
func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	if !r.Committed() {
		w.Header().Set("X-Error-Handler", "app")
	}
	volt.DefaultErrorHandler(w, r, err)
}

// OpsErrors is the /ops scope's own error_handler: nearest wins
// (§V4.4.3), so routes under /ops reach this one, not Errors.
func OpsErrors(w http.ResponseWriter, r *volt.Request, err error) {
	if !r.Committed() {
		w.Header().Set("X-Error-Handler", "ops")
	}
	volt.DefaultErrorHandler(w, r, err)
}

// TagExtra marks the nested scope's appended pipeline (§V4.4.1).
func TagExtra(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "extra")
		next.ServeHTTP(w, r)
	})
}
