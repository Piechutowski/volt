// spec: §scope-loads, a dataset beneath a scope load reads no parameter from the query string under the load's name, which would let the client choose the value the name stands for
// want: parameter "user" of MsRevenueRecent binds from the query string by the name of a scope load above the route, whose value a dataset's route drops
package app

import (
	db
)

scope /me {
	load user CurrentUser
	dataset db.recent [strip: "ms_"]
}
