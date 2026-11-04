package app

import (
	"corpus/db"
	"github.com/Piechutowski/volt"
)

func current(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(1)) }

func first(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(1)) }

func owner(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(2)) }
