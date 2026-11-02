// spec: §route-loads — a load gives the query's arguments in its order and number
// want: takes 1 argument(s), the load gives 2
package app

import (
	db
)

Scope / {
	get /metrics/:id(int) Metrics.Show {
		metric db.MetricGet(id, 1)
	}
}
