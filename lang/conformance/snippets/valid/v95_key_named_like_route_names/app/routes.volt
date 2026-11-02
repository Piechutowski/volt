package app

import (
	db
)

Scope /api {
	resources db.gauges   [default]
	resources db.counters [default]
	resources db.meters   [default]
	resources db.samples  [default]
	resources db.spans    [default]
	resources db.probes   [default]
	resources db.traces   [default]
	get /gauge/:c_(int) db.GaugeGet
}
