// spec: §route-loads, default_limit: is the limit of a request that leaves the key out, so on a route whose limit comes from a path parameter, always present, it governs nothing
// want: limit: is the limit a request that leaves the key out gets, and the route binds no select's limit: parameter from the query string
package app

import (
	db
)

scope / {
	get /top/:n(int) Pages.Top [default_limit: 5] {
		feed db.PostFeed(tag, at, id, n)
	}
}
