package app

import (
	db
)

pipeline api {
	use volt.RequestID
	use BearerAuth
}

scope / [pipe: api, error_handler: Errors] {
	get  /        Home.Index [name: root]
	get  /about   Home.About

	scope /admin [name: admin] {
		get /stats Admin.Stats {
			totals db.UserTotals()
			feed   db.UserFeed()
			latest db.UserLatest(5)
			ten    db.UserLatest(010)
			written db.UserWritten(title, after)
		}
		get /chosen/:email Admin.Chosen {
			chosen db.UserChosen(ids, email)
			wrote  db.UserWritten("draft", after)
		}
		get /picked/:email db.UserChosen [name: picked_by]
		get /newest db.UserLatest [name: newest, limit: 10, max_limit: 50]
		get /newest/page Admin.Newest [name: newest_page, max_limit: 20] {
			rows db.UserLatest(size)
		}
		get /top/:n(int) Admin.Top [max_limit: 100] {
			top db.UserLatest(n)
		}
		get /keys Admin.Keys {
			a db.UserLatest(ctx)
			b db.UserSince(ctx_)
		}
	}

	resources db.users [only: (index, show, create)]

	get /files/:path...          Files.Serve
	get /users/:id(int)/avatar Users.Avatar
	get /users/:id(int)/profile Users.Profile [timeout: 5] {
		user   db.UserGet(id)
		picked db.UserPicked(ids)
		since  db.UserSince(since)
		recent db.UserLatest(count)
	}
	patch /users/:id(int)/email Users.ChangeEmail [body: db.UserUpdate(id), only: (email)]
	post  /signup                Users.Signup      [body: db.UserCreate]
	post  /login                 Session.Login     [body: LoginForm]
	any /ping                    Home.Ping

	scope /me [name: me] {
		load user CurrentUser
		get /      Me.Show
		get /peers db.UserList
		get /picks/:n(int) Me.Picks {
			picks db.UserLatest(n)
		}
		get /latest db.UserLatest
	}

	scope /api [name: api] {
		get    /users            db.UserList
		get    /users/:id(int) db.UserGet
		post   /users            db.UserCreate
		patch  /users/:id(int) db.UserUpdate
		delete /users/:id(int) db.UserDelete
		get    /picked           db.UserPicked
		get    /since            db.UserSince
		get    /totals           db.UserTotals
		get    /feed             db.UserFeed
		get    /latest           db.UserLatest
		get    /written          db.UserWritten
		get    /bydb             db.UserBydb
		get    /roles/:userID(int)/:roleID(int) db.UserRoleGet
		patch  /roles/:userID(int)/:roleID(int) db.UserRoleUpdate
	}

	// A dataset (§datasets): one query route per member of the group select.
	scope /ms {
		dataset db.browse [strip: "ms_"]
	}
	scope /msr [name: msr] {
		dataset db.recent [strip: "ms_", limit: 5, max_limit: 20]
	}

	// Default handlers (§default-handlers): every action is a query route over
	// the table's generated CRUD; no controller to write.
	scope /ref [name: ref] {
		resources db.users [default]
		resources db.tags  [default]
	}

	// The event stream (§event-routes).
	get /events volt.Events
}
