// spec: §route-loads — a load block belongs to a controller route, not an event route
// want: a load block belongs to a controller route
package app

import (
	db
)

scope / {
	get /ordered db.MetricOrdered
	get /events volt.Events {
		all db.MetricOrdered()
	}
}
