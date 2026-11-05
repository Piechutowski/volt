// spec: §query-routes, an optional list parameter is not routable: a missing key and an empty list cannot be told apart in a query string
// want: db.PostPicked takes the optional list parameter "ids", which a route cannot carry
package app

import (
	db
)

scope /api {
	get /picked db.PostPicked
}
