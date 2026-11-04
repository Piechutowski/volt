package app

import (
	itest/blog/db
)

pipeline api {
	use volt.RequestID
	use TagOuter
	use TagInner
}

pipeline extra {
	use TagExtra
}

scope / [pipe: api, error_handler: Errors] {
	get /        Home.Index [name: root]
	get /teapot  Home.Teapot
	any /ping    Home.Ping

	scope /admin [pipe: extra, name: admin] {
		get /stats Admin.Stats
	}

	scope /ops [name: ops, error_handler: OpsErrors] {
		get /fail Ops.Fail
	}

	resources db.users

	get /users/:id(int)/avatar Users.Avatar
	get /users/:id(int)/profile Users.Profile [name: profile, timeout: 5] {
		user db.UserGet(id)
	}
	get /users/:id(int)/card    Users.Card    [name: card]
	get /users/:id(int)/peers/:n(int) Users.Peers [name: peers] {
		user  db.UserGet(id)
		peers db.UserLatest(n)
		more  db.UserLatest(count)
	}
	get /users/:id(int)/both Users.Both [name: both] {
		latest    db.UserLatest(count)
		described db.UserDescribed(bio, count)
	}
	get /users/:id(int)/from Users.From [name: from] {
		profiled db.UserDescribed("%", id)
	}
	patch /users/:id(int)/email Users.ChangeEmail [name: change_email, body: db.UserUpdate(id), only: (email)]
	post  /login                Session.Login     [name: login, body: LoginForm]
	get /files/:path...          Files.Serve

	// Scope loads (§scope-loads): the current user, read from a header,
	// reaches every route beneath, and a query route runs it for the halt.
	// A limit from the path is checked before the scope load runs, one
	// from the query string after it.
	scope /me [name: me] {
		load user CurrentUser
		get /      Me.Show
		get /peers db.UserList
		get /picks/:n(int) Me.Picks {
			picks db.UserLatest(n)
		}
		get /latest db.UserLatest
	}

	get /tags/:name             Tags.Show    [name: tag]
	get /pages/:num(int)        Pages.Show   [name: page]
	get /archive/:stamp(int)  Archive.Show [name: archive]

	// Query routes (§query-routes): generated handlers over the data package.
	scope /api [name: api] {
		get    /users            db.UserList
		get    /users/:id(int) db.UserGet
		post   /users            db.UserCreate
		patch  /users/:id(int) db.UserUpdate
		delete /users/:id(int) db.UserDelete
		get    /picked           db.UserPicked
		get    /since            db.UserSince
		get    /latest           db.UserLatest
		get    /described        db.UserDescribed
		get    /chosen           db.UserChosen
		get    /recent           db.UserLatest [name: recent, limit: 2, max_limit: 3]
		get    /top/:limit(int)  db.UserLatest [name: top, max_limit: 3]
	}

	// A dataset (§datasets): one query route per member of the group select.
	scope /ms {
		dataset db.browse [strip: "ms_"]
	}

	// Default handlers (§default-handlers): every action is a query route over
	// the table's generated CRUD; no controller to write.
	scope /ref [name: ref] {
		resources db.users    [default]
		resources db.tags     [default]
		resources db.profiles [default]
	}

	// The event stream (§event-routes).
	get /events volt.Events
}
