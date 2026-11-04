// spec: §route-loads, default_limit: on a route that binds no limit from the query string governs nothing
// want: limit: is the limit a request that leaves the key out gets, and the route binds no select's limit: parameter from the query string
package app

import (
	db
)

scope / {
	get /home Pages.Home [name: home, default_limit: 10] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
