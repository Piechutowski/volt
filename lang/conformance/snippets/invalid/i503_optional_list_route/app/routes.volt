// spec: §query-routes, an optional list parameter is not routable, from the query string or a path parameter: a missing key and an empty list cannot be told apart in a query string
// want: db.PostPicked takes the optional list parameter "ids", which a route cannot carry: a missing key and an empty list cannot be told apart in a query string, and an absent list matches every row where an empty one matches none; call the select from Go
package app

import (
	db
)

scope /api {
	get /picked db.PostPicked
	get /picked/:ids db.PostPicked [name: picked_by_path]
}
