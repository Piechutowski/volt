// spec: §query-routes, §formats — query routes: selects and default CRUD bound by name, parameters from path, body and query string
package app

import (
	db
)

Scope /api {
	get    /users            db.UserList
	get    /users/:id(int) db.UserGet
	post   /users            db.UserCreate
	patch  /users/:id(int) db.UserUpdate
	put    /users/:id(int) db.UserUpdate [name: replace_user]
	delete /users/:id(int) db.UserDelete
	get    /picked           db.UserPicked
	get    /named            db.UserNamed
	get    /by/:name         db.UserNamed [name: by_name]
}
