// A Volt routing package (docs/spec.md Part II) — the layer above the schema
// kitchen sink next door. One language: this file and kitchen_sink.volt
// parse with the same grammar.
package app

import (
	db
	d2 shared/dicts
)

Pipeline api {
	use volt.RequestID
	use BearerAuth
}

Scope / [pipe: api, error_handler: Errors] {
	get /        Home.Index [name: root]
	any /ping    Home.Ping

	Scope /admin [name: admin] {
		get /stats Admin.Stats
	}

	resources db.users [only: (index, show, create)]

	get /users/:id(int)/avatar Users.Avatar
	get /files/:path...          Files.Serve
}
