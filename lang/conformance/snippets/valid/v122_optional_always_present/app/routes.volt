// spec: §query-routes, §route-loads, a path parameter and a literal bind an optional parameter as present, since each is always present
package app

import (
	db
)

scope /api {
	get /feed/:tag db.PostFeed [name: tagged]
}

scope / {
	get /go Pages.Go [name: go_feed] {
		feed db.PostFeed("go", at, id, 20)
	}
	get /tags/:tag Pages.Tag [name: tag_feed] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
