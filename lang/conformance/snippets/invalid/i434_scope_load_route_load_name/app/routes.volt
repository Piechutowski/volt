// spec: §scope-loads — a route load cannot take a scope load's name
// want: takes the name of a scope load
package app

import (
	db
)

scope / {
	load user CurrentUser
	get /me Me.Show {
		user db.UserGet(1)
	}
}
