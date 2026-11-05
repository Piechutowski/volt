// spec: §route-loads, a literal is always present, so it binds no optional parameter, and the error names the fix, a second select or a required parameter
// want: argument 1 of db.PostFeed binds the optional parameter "tag", which a literal, always present, cannot bind: an optional parameter binds from the query string only; name a query-string key, or load a second select whose parameter is required
package app

import (
	db
)

scope / {
	get /go Pages.Go [name: go_feed] {
		feed db.PostFeed("go", at, id, 20)
	}
}
