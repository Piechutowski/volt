// Package app is the hand-written side of the routes: the controllers
// and the two error handlers routes.volt names, each stamping its name
// on the response so the window shows which scope answered (§scopes).
package app

import (
	"errors"

	"github.com/Piechutowski/volt"
)

// Message is what every handler renders.
type Message struct {
	Text string
}

// Errors answers a failure of any route under the root scope.
func Errors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "Errors")
}

// AdminErrors answers a failure under /admin: the nearest error handler wins.
func AdminErrors(r volt.Request, err error) volt.Response {
	return volt.DefaultErrorHandler(r, err).WithHeader("X-Error-Handler", "AdminErrors")
}

// Pages are the routes of the root scope.
type Pages struct{}

// Hello renders a greeting.
func (Pages) Hello(r volt.Request) volt.Response {
	return volt.Render(Message{Text: "hello from " + r.Route()})
}

// Broken fails, so the root scope's error handler answers.
func (Pages) Broken(r volt.Request) volt.Response {
	return volt.Fail(errors.New("Pages.Broken failed on purpose"))
}

// Admin are the routes of the /admin scope.
type Admin struct{}

// Stats renders a line.
func (Admin) Stats(r volt.Request) volt.Response {
	return volt.Render(Message{Text: "stats from " + r.Route()})
}

// Broken fails, so the /admin scope's error handler answers.
func (Admin) Broken(r volt.Request) volt.Response {
	return volt.Fail(errors.New("Admin.Broken failed on purpose"))
}
