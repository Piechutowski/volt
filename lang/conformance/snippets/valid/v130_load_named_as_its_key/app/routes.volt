// spec: §route-loads, a load's argument may share the load's own name, which binds the query key of that name, since no load's value is read through it
package app

import (
	db
)

scope / {
	get /posts Pages.Posts {
		title db.PostTitled(title)
	}
}
