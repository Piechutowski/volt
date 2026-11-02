// spec: §scope-loads — a scope load name is not reserved
// want: is not a name a handler parameter can take
package app

import (
	db
)

Scope / {
	load body CurrentUser
	get /me Me.Show
}
