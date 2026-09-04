// spec: §V4.8 — i102_query_list_in_path
// want: cannot be a path parameter
package app

import (
	db
)

Scope /api {
	get /picked/:ids db.UserPicked
}
