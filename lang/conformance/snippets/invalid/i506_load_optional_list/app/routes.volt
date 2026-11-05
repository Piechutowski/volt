// spec: §route-loads, §query-routes, an optional list parameter cannot be loaded, whatever its argument, for the reason it cannot be routed
// want: binds the optional list parameter "ids", which a load cannot carry: a missing key and an empty list cannot be told apart in a query string, and an absent list matches every row where an empty one matches none; call the select in the handler with Then
package app

import (
	db
)

scope / {
	get /picked Pages.Picked [name: picked] {
		posts db.PostPicked(ids)
		fixed db.PostPicked(5)
	}
	get /picked/:ids Pages.PickedBy [name: picked_by] {
		posts db.PostPicked(ids)
	}
}
