// spec: §route-loads — err and ok are the shim's own locals
// want: not err or ok
package app

import (
	db
)

scope / {
	get /ordered db.MetricOrdered
	get /metrics Metrics.Index {
		err db.MetricOrdered()
	}
}
