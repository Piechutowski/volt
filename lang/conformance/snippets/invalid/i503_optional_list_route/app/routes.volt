// spec: §query-routes, an optional list parameter is not routable: the query string has no spelling for a present empty list
// want: db.PostPicked takes the optional list parameter "ids", which a route cannot carry
package app

import (
	db
)

scope /api {
	get /picked db.PostPicked
}
