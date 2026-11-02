// spec: §route-loads — a load cannot shadow an import qualifier
// want: takes the name of a package
package app

import (
	db
)

scope / {
	get /ordered db.MetricOrdered
	get /metrics Metrics.Index {
		db db.MetricOrdered()
	}
}
