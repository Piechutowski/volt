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
}
