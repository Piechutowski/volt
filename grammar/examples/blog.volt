// A Volt routing package (SPEC.md §V) — the layer above the DBML
// kitchen sink next door. One language: this file and kitchen_sink.dbml
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

	get /users/:id(int32)/avatar Users.Avatar
	get /files/:path...          Files.Serve
}
