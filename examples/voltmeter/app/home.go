package app

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/voltmeter/assets"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/nao/rt"
)

// Home answers the root and the ping (spec §routes).
type Home struct{}

// Index sends a browser to the landing page.
func (Home) Index(r volt.Request) volt.Response {
	return volt.Redirect(PathAsset("voltmeter.html"))
}

// Ping answers any verb with the method it was asked and the time the
// shim captured.
func (Home) Ping(r volt.Request) volt.Response {
	return volt.Render(Pong{Method: r.Method(), At: r.Now()})
}

// Assets serves the embedded files (spec §requests-and-responses,
// volt.File).
type Assets struct{}

// Serve is the file at path in the embedded directory, a 404 when there
// is none.
func (Assets) Serve(r volt.Request, path string) volt.Response {
	return volt.File(assets.FS(), path)
}

// Session signs a member in.
type Session struct{}

// SignIn finds the member by email and sets the cookie CurrentUser
// reads; an unknown email is a 401, decided by Rescue on the lookup.
func (Session) SignIn(r volt.Request, body SignIn) volt.Response {
	if err := db.EmailShape(body.Email); err != nil {
		return volt.Reject(http.StatusUnprocessableEntity, "check the email").WithDetails(fieldErrors("EmailShape", "email", err.Error()))
	}
	return volt.Then(db.UserGetByEmail(body.Email), func(u db.User) volt.Response {
		return volt.Render(u).WithCookie(http.Cookie{
			Name:     UserCookie,
			Value:    strconv.Itoa(u.ID),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}).Rescue(func(err error) volt.Response {
		if errors.Is(err, rt.ErrNotFound) {
			return volt.Reject(http.StatusUnauthorized, "no member signs in as "+body.Email)
		}
		return volt.Fail(err)
	})
}

// Me answers who the member scope's load found.
type Me struct{}

// Show is the signed-in member.
func (Me) Show(r volt.Request, user db.User) volt.Response {
	return volt.Render(user)
}

const probeSQL = `(WITH RECURSIVE voltmeter_probe(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM voltmeter_probe WHERE x < ?) SELECT max(x) FROM voltmeter_probe) > 0`

// Probe times the database itself: the collector calls it to measure
// the machine, and its one-second timeout bounds what a caller can ask.
type Probe struct{}

// Count makes the database count to the steps query parameter while it
// counts the members; a count the timeout cuts off is a 504.
func (Probe) Count(r volt.Request) volt.Response {
	steps, err := volt.QueryParam[int](r, "steps")
	if err != nil {
		return volt.Fail(err)
	}
	if steps < 1 {
		return volt.Reject(http.StatusBadRequest, "steps must be at least 1")
	}
	return volt.Then(db.UserCount(rt.Raw[db.User](probeSQL, steps)), func(n int64) volt.Response {
		return volt.Render(Counted{Steps: steps, Rows: n})
	})
}
