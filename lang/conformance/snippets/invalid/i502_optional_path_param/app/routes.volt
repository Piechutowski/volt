// spec: §query-routes, an optional parameter binds from the query string only; a path segment is always present, and the error names the fix, a second select or a required parameter
// want: parameter "tag" of db.PostFeed is optional, and a path segment is always present; pass it in the query string, or route a second select whose parameter is required
package app

import (
	db
)

scope /api {
	get /feed/:tag db.PostFeed
}
