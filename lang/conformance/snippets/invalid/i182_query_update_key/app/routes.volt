// spec: §query-routes (i163_query_update_key)
// want: must be a path parameter of the route
package app

import (
	db
)

Scope /api {
	patch /users db.UserUpdate
}
