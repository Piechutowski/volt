package app

import (
	"strconv"

	"corpus/db"
	"github.com/Piechutowski/volt"
)

func CurrentUser(r volt.Request) volt.Load[db.User] {
	id, err := strconv.Atoi(r.Header().Get("X-User"))
	if err != nil {
		return volt.Halt[db.User](volt.Reject(401, "sign in"))
	}
	return volt.LoadWith(db.UserGet(id))
}

func Tenant(r volt.Request) volt.Load[string] { return volt.Halt[string](volt.NoContent()) }

func Plain(r volt.Request) volt.Response { return volt.NoContent() }

func Errors(r volt.Request, err error) volt.Response { return volt.DefaultErrorHandler(r, err) }
