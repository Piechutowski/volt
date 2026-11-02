// spec: §route-loads — a load names a generated query of the data package
// want: no generated query db.MetricRecent
package app

import (
	db
)

Scope / {
	get /charts Pages.Charts {
		metrics db.MetricRecent()
	}
}
