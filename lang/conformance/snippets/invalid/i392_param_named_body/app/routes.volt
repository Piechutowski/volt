// spec: §request-bodies — body is a reserved name a path parameter cannot take
// want: path parameter "body" collides
package app

import (
	db
)

Scope / {
	get /ordered db.MetricOrdered
	get /metrics/:body Metrics.Show
}
