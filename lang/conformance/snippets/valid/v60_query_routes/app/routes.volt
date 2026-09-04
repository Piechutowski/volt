// spec: §V4.8, §V4.9 — query routes: selects and default CRUD bound by name, parameters from path, body and query string
package app

import (
	db
)

Scope /api {
	get    /users            db.UserList
	get    /users/:id(int32) db.UserGet
	post   /users            db.UserCreate
	patch  /users/:id(int32) db.UserUpdate
	put    /users/:id(int32) db.UserUpdate [name: replace_user]
	delete /users/:id(int32) db.UserDelete
	get    /picked           db.UserPicked
	get    /named            db.UserNamed
	get    /by/:name         db.UserNamed [name: by_name]
}
