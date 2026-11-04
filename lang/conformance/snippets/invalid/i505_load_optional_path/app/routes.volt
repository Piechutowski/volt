// spec: §route-loads, a path parameter is always present, so it binds no optional parameter
// want: argument 1 of db.PostFeed binds the optional parameter "tag", which a path parameter, always present, cannot bind
package app

import (
	db
)

scope / {
	get /tags/:tag Pages.Tag [name: tag_feed] {
		feed db.PostFeed(tag, at, id, 20)
	}
}
