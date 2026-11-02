// spec: §route-loads — a load reads; a write is not a load
// want: writes; a load reads
package app

import (
	db
)

scope / {
	get /metrics/:id(int) Metrics.Show {
		gone db.MetricDelete(id)
	}
}
