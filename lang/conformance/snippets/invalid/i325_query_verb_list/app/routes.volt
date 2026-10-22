// spec: §query-routes, a List is routed with get: the generated method fixes the verb, and any other is an error naming get
// want: db.PostList is routed with get, not post: the generated method a query route binds fixes its verb (§query-routes)
package app

import (
	db
)

Scope /api {
	post /posts db.PostList
}
