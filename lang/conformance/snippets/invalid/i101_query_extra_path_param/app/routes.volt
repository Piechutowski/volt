// spec: §V4.8 — i101_query_extra_path_param
// want: is not a parameter of
package app

import (
	db
)

Scope /api {
	get /users/:x(int32) db.UserList
}
