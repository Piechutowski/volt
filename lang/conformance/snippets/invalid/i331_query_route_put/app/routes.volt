// spec: §routes, §query-routes, put on an Update is the put error alone: the line is read as patch, so the verb an Update takes raises nothing more
// want: put is not a route verb: the partial update is patch, so write patch /posts/:id(int) db.PostUpdate (§routes)
package app

import (
	db
)

Scope /api {
	put /posts/:id(int) db.PostUpdate
}
