// spec: §route-loads, a literal is always present, so it binds no optional parameter
// want: argument 1 of db.PostFeed binds the optional parameter "tag", which a literal, always present, cannot bind
package app

import (
	db
)

scope / {
	get /go Pages.Go [name: go_feed] {
		feed db.PostFeed("go", at, id, 20)
	}
}
