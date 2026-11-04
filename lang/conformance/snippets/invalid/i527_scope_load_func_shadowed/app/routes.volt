// spec: §scope-loads, a scope load beneath another calls no function named as that load, whose value the shim holds under the name
// want: scope load "user" calls account, the name of a load of this scope or one above it
package app

import (
	db
)

scope / {
	load account owner
	scope /me {
		load user account
		get / Me.Show
	}
}
