// spec: §route-loads, one query key binds one type across the loads of a route, the one the generated client sends it as
// want: load b binds the query key "q" as int, and load a binds it as rt.Null[string]
package app

import (
	db
)

scope / {
	get /mixed Pages.Mixed {
		a db.PostTitled(q)
		b db.PostScored(q)
	}
}
