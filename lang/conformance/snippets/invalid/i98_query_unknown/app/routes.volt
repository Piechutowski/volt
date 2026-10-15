// spec: §query-routes — i98_query_unknown
// want: no generated query
package app

import (
	db
)

Scope /api {
	get /users db.UserLst
}
