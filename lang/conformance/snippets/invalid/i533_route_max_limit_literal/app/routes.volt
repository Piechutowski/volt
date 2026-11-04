// spec: §route-loads, max_limit: on a route that binds a select's limit: parameter from no query key and no path parameter, a literal alone, governs nothing
// want: max_limit: is the most a request may ask, and the route binds no select's limit: parameter from the query string or a path parameter
package app

import (
	db
)

scope /api {
	get /s Pages.S [max_limit: 2] {
		rows db.PostTop(5)
	}
}
