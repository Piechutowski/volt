package app

import (
	itest/blog/db
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

	get /users/:id(int)/avatar Users.Avatar
	get /files/:path...          Files.Serve

	get /tags/:name             Tags.Show    [name: tag]
	get /pages/:num(int)        Pages.Show   [name: page]
	get /archive/:stamp(int)  Archive.Show [name: archive]

	// Query routes (§query-routes): generated handlers over the data package.
	Scope /api [name: api] {
		get    /users            db.UserList
		get    /users/:id(int) db.UserGet
		post   /users            db.UserCreate
		patch  /users/:id(int) db.UserUpdate
		delete /users/:id(int) db.UserDelete
		get    /picked           db.UserPicked
	}

	// A dataset (§datasets): one query route per member of the group select.
	Scope /ms {
		dataset db.browse [strip: 'ms_']
	}

	// Default handlers (§default-handlers): every action is a query route over
	// the table's generated CRUD; no controller to write.
	Scope /ref [name: ref] {
		resources db.users [default]
	}

	// The event stream (§event-routes).
	get /events volt.Events
}
