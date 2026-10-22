// spec: §routes, §query-routes, put on a query route is the put error alone, naming the verb the generated method fixes: the line is read as that verb, so nothing more is raised, and a query that resolves to nothing gets the put error naming patch beside its own error
// want: put is not a route verb: the partial update is patch, so write patch /posts/:id(int) db.PostUpdate (§routes)
package app

import (
	db
)

Scope /api {
	put /posts/:id(int) db.PostUpdate
	put /posts          db.PostList
	put /drafts         db.PostCreate
	put /posts/:id(int)/x db.PostDelete
	put /old            db.PostFetch
}
