// spec: §route-loads — an event route takes no timeout
// want: an event route takes no timeout
package app

import (
	db
)

scope / {
	get /events volt.Events [timeout: 5]
	get /ordered db.MetricOrdered
}
