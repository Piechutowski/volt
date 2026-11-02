// spec: §request-bodies — body: names a write, Create or Update, never a read
// want: is not a write a body binds to
package app

import (
	db
)

Scope / {
	post /metrics Metrics.Create [body: db.MetricGet(1)]
}
