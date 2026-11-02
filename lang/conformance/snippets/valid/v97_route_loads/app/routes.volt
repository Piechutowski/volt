// spec: §route-loads, a controller route loads reads of a data package before its handler, bound from the path, the query string and literals, under a timeout
package app

import (
	db
)

scope / {
	get /charts Pages.Charts [timeout: 5] {
		metrics  db.MetricOrdered()
		settings db.SettingGet(1)
	}
	get /metrics/:id(int) Metrics.Show {
		metric db.MetricGet(id)
		peers  db.MetricByUnit(unit, ids)
		other  db.MetricByUnit("ms", ids)
	}
}
