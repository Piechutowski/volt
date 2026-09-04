// spec: §V4.8 — a path parameter must spell the query parameter's Go type
// want: spell it :id(int32)
package app

import (
	db
)

Scope /api {
	get /users/:id(int64) db.UserGet
}
