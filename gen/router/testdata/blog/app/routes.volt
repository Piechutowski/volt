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
	get /users/:id(int)/avatar Users.Avatar
	any /ping                    Home.Ping

	Scope /api [name: api] {
		get    /users            db.UserList
		get    /users/:id(int) db.UserGet
		post   /users            db.UserCreate
		patch  /users/:id(int) db.UserUpdate
		delete /users/:id(int) db.UserDelete
		get    /picked           db.UserPicked
		get    /since            db.UserSince
		get    /roles/:userID(int)/:roleID(int) db.UserRoleGet
		patch  /roles/:userID(int)/:roleID(int) db.UserRoleUpdate
	}

	// A dataset (§datasets): one query route per member of the group select.
	Scope /ms {
		dataset db.browse [strip: "ms_"]
	}

	// Default handlers (§default-handlers): every action is a query route over
	// the table's generated CRUD; no controller to write.
	Scope /ref [name: ref] {
		resources db.users [default]
		resources db.tags  [default]
	}

	// The event stream (§event-routes).
	get /events volt.Events
}
