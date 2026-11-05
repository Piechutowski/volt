// spec: §query-routes, §route-loads, an optional parameter binds from the query string, absent when the key is missing, on a query route and in a load
package app

import (
	db
)

scope /api {
	get /feed db.PostFeed
	get /home Pages.Home [name: home] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
