// spec: §query-routes, §routes, a read is routed with get, which ServeMux also serves for HEAD: head on a query route is an error naming get, while a controller route keeps head
// want: db.PostGet is routed with get, not head: the generated method a query route binds fixes its verb, and ServeMux answers HEAD on a get route (§query-routes)
package app

import (
	db
)

Scope /api {
	head /posts/:id(int) db.PostGet
	head /posts          Posts.Probe
}
