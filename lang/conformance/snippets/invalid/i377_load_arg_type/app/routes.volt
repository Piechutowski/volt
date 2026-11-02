// spec: §route-loads — a path parameter spells the query parameter's Go type, a literal its type, and a list never comes from the path
// want: path parameter "id" is string but db.MetricGet takes int
// want: argument 1 of db.MetricByUnit is string, which the number 3 does not spell
// want: list parameter "ids" of db.MetricByUnit cannot come from a path parameter
package app

import (
	db
)

Scope / {
	get /metrics/:id Metrics.Show {
		metric db.MetricGet(id)
	}
	get /units/:ids(int) Units.Show {
		peers db.MetricByUnit(3, ids)
	}
}
