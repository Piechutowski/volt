package app

import (
	"corpus/db"
	"github.com/Piechutowski/volt"
)

func account(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(1)) }

func owner(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(2)) }
