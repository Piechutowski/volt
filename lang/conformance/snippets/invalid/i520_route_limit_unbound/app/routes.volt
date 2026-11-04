// spec: §route-loads, limit: on a route that binds no limit from the query string governs nothing
// want: limit: sets a limit the route binds from the query string, and the route binds none
package app

import (
	db
)

scope / {
	get /home Pages.Home [name: home, limit: 10] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
