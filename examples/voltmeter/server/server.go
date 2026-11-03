// Package server wires Voltmeter's generated router to a database and an
// event broker, the one place the application's effects meet the world
// (spec §values-and-effects). cmd/voltmeter serves Handler and the end to
// end tests drive the same function on a database of their own.
package server

import (
	"net/http"
	"strings"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/nao/run"
)

// EventChanged is the name of the event Publish sends after a write.
const EventChanged = "changed"

// Change is the payload of EventChanged: which route wrote, so a client
// refetches what it shows.
type Change struct {
	Method  string `json:"method"`
	Route   string `json:"route"`
	Handler string `json:"handler"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
}

// Handler is Voltmeter's whole HTTP surface on d, with b as the event
// broker: the generated router over every controller, wrapped by
// Publish.
func Handler(d run.DB, b *volt.Broker) http.Handler {
	c := app.NewControllers()
	c.DB = d
	c.Events = b
	return Publish(b, app.NewRouter(c))
}

// Publish sends EventChanged on b after every POST, PATCH or DELETE that
// answers 2xx. A handler returns values and cannot publish, and the
// runtime has no effect that does, so the broker is driven from here,
// around the router.
func Publish(b *volt.Broker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status < 200 || sw.status > 299 {
			return
		}
		_ = b.Publish(EventChanged, Change{
			Method:  r.Method,
			Route:   r.Pattern,
			Handler: handlerOf(r.Pattern),
			Path:    r.URL.Path,
			Status:  sw.status,
		})
	})
}

func handlerOf(pattern string) string {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		method, path = "", pattern
	}
	for _, rt := range app.Table {
		if rt.Method != method || rt.Pattern != path {
			continue
		}
		if rt.Query != "" {
			return rt.Query
		}
		return rt.Controller + "." + rt.Action
	}
	return ""
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
