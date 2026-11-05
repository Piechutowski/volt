// spec: §route-loads, a load is not named after a predeclared Go identifier, which the shim's closure taking its value would shadow
// want: load "string" is not a name a handler parameter can take
package app

import (
	db
)

scope / {
	get /me Me.Show {
		string db.UserGet(1)
		row    db.UserGet(2)
	}
}
