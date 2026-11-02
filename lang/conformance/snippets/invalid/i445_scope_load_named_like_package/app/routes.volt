// spec: §scope-loads — a scope load cannot shadow the package's own name
// want: takes the name of a package
package app

import (
	db
)

Scope / {
	load app CurrentUser
	get /me Me.Show
	get /ordered db.MetricOrdered
}
