// spec: §route-loads, limit: is the default of a limit the route binds from the query string and max_limit: its most, on a query route and in a load
package app

import (
	db
)

scope /api {
	get /feed db.PostFeed [limit: 10, max_limit: 50]
	get /home Pages.Home [name: home, max_limit: 20] {
		feed db.PostFeed(tag, at, id, size)
	}
}
