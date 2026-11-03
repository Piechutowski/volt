// Package app is Voltmeter's routes package: routes.volt declares the
// pipelines, scopes, resources and datasets, and the Go files beside it
// hold what the routes name by bare name, the plugs, the error handlers,
// the scope loads, the struct bodies and the controllers (spec
// §scopes-and-routes). Every controller is an empty struct whose actions
// are pure functions from a request value to a response value
// (spec §requests-and-responses).
package app

import (
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

// UserCookie is the cookie a sign-in sets, read by CurrentUser beside
// the X-User header; Volt has no sessions yet, so it carries the bare id.
const UserCookie = "voltmeter_user"

// Served is the api pipeline's own plug (spec §pipelines): it names the
// server and records the pipeline in X-Pipeline, so the order of plugs
// is visible on every response.
func Served(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "voltmeter")
		w.Header().Add("X-Pipeline", "api")
		next.ServeHTTP(w, r)
	})
}

// Vary is the member pipeline's plug (spec §pipelines): a member's
// answers depend on who asks, so caches key on X-User and keep them
// private.
func Vary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "X-User")
		w.Header().Set("Cache-Control", "private")
		w.Header().Add("X-Pipeline", "member")
		next.ServeHTTP(w, r)
	})
}

// Errors is the root error handler (spec §scopes): the default Problem
// in the negotiated format, marked so a client can tell which handler
// answered.
func Errors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "api")
}

// ReportErrors is the error handler of the report scope (spec §scopes):
// a report is read in a browser, so a failure is a page, not a Problem.
func ReportErrors(r volt.Request, err error) volt.Response {
	status := volt.DefaultErrorHandler(r, err).Status()
	msg := http.StatusText(status)
	if p, ok := volt.ProblemOf(err); ok {
		msg = p.Message
	}
	return volt.HTML(ErrorPage(status, msg)).WithStatus(status).WithHeader("X-Error-Handler", "report")
}

// CurrentUser is the member scope's load (spec §scope-loads): the user
// the X-User header or the sign-in cookie names, or a 401 that halts
// the request before anything beneath runs.
func CurrentUser(r volt.Request) volt.Load[db.User] {
	raw := r.Header().Get("X-User")
	if raw == "" {
		hr := http.Request{Header: r.Header()}
		if c, err := hr.Cookie(UserCookie); err == nil {
			raw = c.Value
		}
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		return volt.Halt[db.User](volt.Reject(http.StatusUnauthorized, "sign in: send X-User or the "+UserCookie+" cookie"))
	}
	return volt.LoadWith(db.UserGet(id))
}

// OwnedBoard is the board scope's load (spec §scope-loads), nested under
// CurrentUser: the dashboard the path names. Whether the member owns it
// is the handler's to judge, since a load sees the request alone.
func OwnedBoard(r volt.Request) volt.Load[db.Dashboard] {
	id, err := strconv.Atoi(r.PathValue("board"))
	if err != nil {
		return volt.Halt[db.Dashboard](volt.Reject(http.StatusNotFound, "no such dashboard"))
	}
	return volt.LoadWith(db.DashboardGet(id))
}

// NewControllers is every controller of the package; the program that
// serves them sets DB and Events (spec §query-routes, §event-routes).
func NewControllers() Controllers {
	return Controllers{
		Assets:     Assets{},
		Budgets:    Budgets{},
		Dashboards: Dashboards{},
		Home:       Home{},
		Me:         Me{},
		Probe:      Probe{},
		Reports:    Reports{},
		Runs:       Runs{},
		Samples:    Samples{},
		Session:    Session{},
		Thesis:     Thesis{},
		Uploads:    Uploads{},
		Widgets:    Widgets{},
	}
}
