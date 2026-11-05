// spec: §scope-loads, a query route beneath a scope load reads no parameter from the query string under the load's name, which would let the client choose the value the name stands for
// want: parameter "user" of PostBy binds from the query string by the name of a scope load above the route
package app

import (
	db
)

scope /me {
	load user CurrentUser
	get /posts db.PostBy
}
