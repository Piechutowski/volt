package app

import (
	db
)

Pipeline api {
	use volt.RequestID
	use BearerAuth
}

Scope / [pipe: api, error_handler: Errors] {
	get  /        Home.Index [name: root]
	get  /about   Home.About

	Scope /admin [name: admin] {
		get /stats Admin.Stats
	}

	resources db.users [only: (index, show, create)]

	get /files/:path...          Files.Serve
	get /users/:id(int32)/avatar Users.Avatar
	any /ping                    Home.Ping

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

	// Default handlers (§V5.5): every action is a query route over
	// the table's generated CRUD; no controller to write.
	Scope /ref [name: ref] {
		resources db.users [default]
	}

	// The event stream (§V4.11).
	get /events volt.Events
}
