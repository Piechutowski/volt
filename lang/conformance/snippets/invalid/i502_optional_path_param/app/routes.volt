// spec: §query-routes, an optional parameter binds from the query string only; a path segment is always present
// want: parameter "tag" of db.PostFeed is optional, and a path segment is always present
package app

import (
	db
)

scope /api {
	get /feed/:tag db.PostFeed
}
