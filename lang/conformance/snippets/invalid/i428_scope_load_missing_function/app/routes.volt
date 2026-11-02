// spec: §scope-loads — a scope load names a function of the package
// want: no function Nobody in package app
package app

import (
	db
)

scope / {
	load user Nobody
	get /me Me.Show
}
