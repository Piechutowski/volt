// spec: §route-loads, an argument naming a scope load reads the query string, so it is an error
// want: argument 1 of db.UserGet names "user", a scope load above the route
package app

import (
	db
)

scope / {
	load user CurrentUser
	get /me Me.Show {
		row db.UserGet(user)
	}
}
