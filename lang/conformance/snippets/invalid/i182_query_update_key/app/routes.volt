// spec: §query-routes (the key of an update is a path parameter)
// want: must be a path parameter of the route
package app

import (
	db
)

Scope /api {
	patch /users db.UserUpdate
}
