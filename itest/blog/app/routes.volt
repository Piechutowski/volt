package app

import (
	db
)

Pipeline api {
	use volt.RequestID
	use TagOuter
	use TagInner
}

Pipeline extra {
	use TagExtra
}

Scope / [pipe: api, error_handler: Errors] {
	get /        Home.Index [name: root]
	get /teapot  Home.Teapot
	any /ping    Home.Ping

	Scope /admin [pipe: extra, name: admin] {
		get /stats Admin.Stats
	}

	Scope /ops [name: ops, error_handler: OpsErrors] {
		get /fail Ops.Fail
	}

	resources db.users

	get /users/:id(int32)/avatar Users.Avatar
	get /files/:path...          Files.Serve

	get /tags/:name             Tags.Show    [name: tag]
	get /pages/:num(int)        Pages.Show   [name: page]
	get /archive/:stamp(int64)  Archive.Show [name: archive]
}
