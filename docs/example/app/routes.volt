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

Dataset da [from: db.group(DA), pipe: api, formats: (html, json, gob)] {
	path: strip('da_')        // da_r_r → /da/r_r
	key:  idpk
	ops:  (list, create, update, delete)

	// Override exactly one operation on one table; everything else stays
	// generated. App.DaRRList must exist with the right signature or the
	// build fails (volt_handlers.go interface).
	da_r_r [list: App.DaRRList]
}
