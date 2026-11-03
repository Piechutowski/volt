// spec: §route-loads, a load argument of type time.Time binds from the query string, and the router imports time for it without a timeout to need it
package app

import (
	db
)

scope / {
	get /recent Metrics.Recent {
		recent db.MetricSince(since)
	}
}
