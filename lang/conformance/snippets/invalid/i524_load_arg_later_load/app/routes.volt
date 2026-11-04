// spec: §route-loads, an argument naming a load the route declares after it reads the query string too, so it is an error
// want: argument 1 of db.UserGet names "later", a load of the route,
package app

import (
	db
)

scope / {
	get /me Me.Show {
		first db.UserGet(later)
		later db.UserGet(1)
	}
}
