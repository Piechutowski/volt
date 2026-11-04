// spec: §route-loads, a default above the most would refuse every bare request
// want: default_limit: 60 is above max_limit: 50
package app

import (
	db
)

scope /api {
	get /feed db.PostFeed [default_limit: 60, max_limit: 50]
}
