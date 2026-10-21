// spec: §route-paths (cs is the body parameter of the generated client)
// want: reserved: w, r, opts, volt, cs
package app

import (
	db
)

Scope /api {
	get   /marks          db.MarkList
	patch /marks/:cs(int) db.MarkUpdate
}
