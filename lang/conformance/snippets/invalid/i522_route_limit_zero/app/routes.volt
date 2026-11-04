// spec: §route-loads, default_limit: and max_limit: take a positive integer of rows
// want: max_limit: takes a positive integer of rows
package app

import (
	db
)

scope /api {
	get /feed db.PostFeed [max_limit: 0]
}
