package app

import (
	db
)

Pipeline api {
	use volt.RequestID
	use volt.Logger
	use app.BearerAuth        // yours — middleware.go, std contract
}

Scope / [pipe: api, error_handler: app.Errors] {
	get /health  Health.Check
}

Dataset ms [from: db.group(MS), pipe: api, formats: (html, json, gob)] {
	path: strip('ms_')        // ms_revenue → /ms/revenue
	key:  id
	ops:  (list, create, update, delete)

	// Override exactly one operation on one table; everything else stays
	// generated. App.MsRevenueList must exist with the right signature or
	// the build fails (volt_handlers.go interface).
	ms_revenue [list: App.MsRevenueList]
}
