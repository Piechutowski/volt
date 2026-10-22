// spec: §query-routes, an Update is routed with patch, the partial update: post is an error naming patch, though both carry a body
// want: db.PostUpdate is routed with patch, not post
package app

import (
	db
)

Scope /api {
	post /posts/:id(int) db.PostUpdate
}
