// spec: §scope-loads, a scope load may call a function named as itself or as a later load of its scope, whose value the shim holds only after the call
package app

import (
	db
)

scope / {
	load current current
	load account owner
	load owner first
	get /me Me.Show
}
