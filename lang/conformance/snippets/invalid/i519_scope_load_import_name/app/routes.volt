// spec: §route-loads, a load is not named like a package the route files import, which its closure would shadow
// want: load "time" is not a name a handler parameter can take
package app

import (
	db
)

scope / {
	load time CurrentUser
	get /users db.UserList
}
