// spec: §route-loads, §query-routes, an optional list parameter cannot be loaded, for the reason it cannot be routed
// want: binds the optional list parameter "ids", which a load cannot carry
package app

import (
	db
)

scope / {
	get /picked Pages.Picked [name: picked] {
		posts db.PostPicked(ids)
	}
}
