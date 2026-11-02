// spec: §request-bodies — body is a reserved name a load cannot take
// want: is not a name a handler parameter can take
package app

import (
	db
)

scope / {
	get /ordered db.MetricOrdered
	get /metrics Metrics.Index {
		body db.MetricOrdered()
	}
}
