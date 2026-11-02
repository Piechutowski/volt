// spec: §query-routes — i101_query_extra_path_param
// want: is not a parameter of
package app

import (
	db
)

scope /api {
	get /users/:x(int) db.UserList
}
