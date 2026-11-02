// spec: §scope-loads, a scope loads a value for every route beneath it, controller and query routes alike, nested scopes appending
package app

import (
	db
)

scope / [error_handler: Errors] {
	get /ping Home.Ping
	scope /me [name: me] {
		load user CurrentUser
		get /          Me.Show
		get /peers     db.UserList
		scope /admin {
			load tenant Tenant
			get /stats Admin.Stats
		}
	}
}
