// spec: §predicates, §query-routes, a parameter named q routes and loads
package app

import (
	db
)

scope /api {
	get /search db.PostSearch
	get /words/:q db.WordGet
	get /find Pages.Find [name: find] {
		posts db.PostSearch(q)
	}
}
