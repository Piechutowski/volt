// spec: §route-loads — a load block belongs to a controller route, whose handler is hand-written
// want: a load block belongs to a controller route
package app

import (
	db
)

scope / {
	get /metrics db.MetricList {
		settings db.SettingGet(1)
	}
}
