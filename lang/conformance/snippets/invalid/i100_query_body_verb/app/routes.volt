// spec: §query-routes, a Create is routed with post: a get naming it, which would carry no body, is an error naming post
// want: db.UserCreate is routed with post, not get
package app

import (
	db
)

Scope /api {
	get /users db.UserCreate
}
