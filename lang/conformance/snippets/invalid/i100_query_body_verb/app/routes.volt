// spec: §V4.8 — i100_query_body_verb
// want: takes a request body
package app

import (
	db
)

Scope /api {
	get /users db.UserCreate
}
