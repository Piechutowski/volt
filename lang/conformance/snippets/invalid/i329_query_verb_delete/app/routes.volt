// spec: §query-routes, a Delete is routed with delete: a get that deletes is an error naming delete
// want: db.PostDelete is routed with delete, not get
package app

import (
	db
)

Scope /api {
	get /posts/:id(int) db.PostDelete
}
