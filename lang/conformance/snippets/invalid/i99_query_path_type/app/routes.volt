// spec: §query-routes — a path parameter must spell the query parameter's Go type
// want: spell it :id(int)
package app

import (
	db
)

Scope /api {
	get /users/:id db.UserGet
}
