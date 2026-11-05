// spec: §route-loads, an argument naming another load of the route reads the query string, so it is an error
// want: argument 1 of db.UserGet names "first", a load of the route,
package app

import (
	db
)

scope / {
	get /me Me.Show {
		first db.UserGet(1)
		again db.UserGet(first)
	}
}
