// spec: §query-routes, §route-loads, an optional list parameter binds from the repeated key, absent when the key is missing, on a query route and in a load
package app

import (
	db
)

scope /api {
	get /picked db.PostPicked
	get /page Pages.Picked [name: picked_page] {
		posts db.PostPicked(ids)
	}
}
