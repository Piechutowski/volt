// spec: §route-loads, §scope-loads, a load is named by a lower-case Go identifier and so by no Go keyword, which the generated handler and shim could not declare
// want: load "range" is not a name a handler parameter can take
package app

import (
	db
)

scope / {
	get /me Me.Show {
		range db.UserGet(1)
		row   db.UserGet(2)
	}
}

scope /u {
	load func CurrentUser
	get / Me.Index
}
