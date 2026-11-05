// spec: §route-loads, a path parameter is always present, so it binds no optional parameter, and the error names the fix, a second select or a required parameter
// want: argument 1 of db.PostFeed binds the optional parameter "tag", which a path parameter, always present, cannot bind: an optional parameter binds from the query string only; name a query-string key, or load a second select whose parameter is required
package app

import (
	db
)

scope / {
	get /tags/:tag Pages.Tag [name: tag_feed] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
