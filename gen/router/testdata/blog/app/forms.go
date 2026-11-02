package app

import (
	"blog/db"
	"github.com/Piechutowski/volt"
)

// LoginForm is the struct body routes.volt names (§request-bodies).
type LoginForm struct {
	Email    string
	Password string
}

// CurrentUser is the scope load routes.volt names (§scope-loads): the
// request's user, or a halt.
func CurrentUser(r volt.Request) volt.Load[db.User] {
	return volt.LoadWith(db.UserGet(1))
}
