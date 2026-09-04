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

	get /users/:id(int32)/avatar Users.Avatar
	get /files/:path...          Files.Serve

	get /tags/:name             Tags.Show    [name: tag]
	get /pages/:num(int)        Pages.Show   [name: page]
	get /archive/:stamp(int64)  Archive.Show [name: archive]

	// Query routes (§V4.8): generated handlers over the data package.
	Scope /api [name: api] {
		get    /users            db.UserList
		get    /users/:id(int32) db.UserGet
		post   /users            db.UserCreate
		patch  /users/:id(int32) db.UserUpdate
		delete /users/:id(int32) db.UserDelete
		get    /picked           db.UserPicked
	}

	// A dataset (§V13): one query route per member of the group select.
	Scope /ms {
		dataset db.browse [strip: 'ms_']
	}
}
