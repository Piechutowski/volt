// spec: §route-loads — a load's name is a handler parameter: not r, not a path parameter, not another load
// want: load "id" takes the name of a path parameter
// want: load "r" is not a name a handler parameter can take
// want: load "metric" is declared twice
package app

import (
	db
)

Scope / {
	get /metrics/:id(int) Metrics.Show {
		id     db.MetricGet(id)
		r      db.MetricOrdered()
		metric db.MetricGet(id)
		metric db.MetricOrdered()
	}
}
