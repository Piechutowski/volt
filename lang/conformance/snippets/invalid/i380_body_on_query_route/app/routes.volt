// spec: §request-bodies — a body belongs to a controller route, whose handler is hand-written
// want: body: belongs to a controller route
package app

import (
	db
)

Scope / {
	post /metrics db.MetricCreate [body: db.MetricCreate]
}
