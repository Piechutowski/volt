package app

import (
	db
)

Pipeline api {
	use volt.RequestID
}

// Two `resources` lines below replace 14 hand-written route lines.
Scope / [pipe: api, error_handler: Errors] {
	get /health Health.Check

	// The full Rails-7 set: index, new, create, show, edit,
	// update (PATCH *and* PUT), delete — 8 routes.
	resources db.Post

	// Nested scope + [api]: no HTML form actions (new/edit), and
	// `name: api` prefixes the generated helpers (PathAPIComment…).
	Scope /api [name: api] {
		resources db.Comment [api, except: (delete)]
	}
}
