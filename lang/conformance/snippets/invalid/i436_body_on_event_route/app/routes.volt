// spec: §request-bodies — a body belongs to a controller route, not an event route
// want: body: belongs to a controller route
package app

import (
	db
)

Scope / {
	get /events volt.Events [body: db.MetricCreate]
	get /ordered db.MetricOrdered
}
