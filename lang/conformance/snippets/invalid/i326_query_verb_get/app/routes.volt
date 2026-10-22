// spec: §query-routes, a Get is routed with get: a delete line naming it is an error naming get, so no GET reads under another verb
// want: db.PostGet is routed with get, not delete
package app

import (
	db
)

Scope /api {
	delete /posts/:id(int) db.PostGet
}
