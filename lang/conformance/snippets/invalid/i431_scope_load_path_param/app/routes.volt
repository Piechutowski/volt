// spec: §scope-loads — a path parameter cannot take a scope load's name
// want: takes the name of a scope load
package app

import (
	db
)

scope / {
	load user CurrentUser
	get /users/:user Users.Show
}
