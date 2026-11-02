// spec: §request-bodies — a create takes no argument
// want: takes no argument: a create loads no row
package app

import (
	db
)

scope / {
	post /metrics Metrics.Create [body: db.MetricCreate(1)]
}
