// spec: §scope-loads — a scope load names its function bare
// want: written by its bare name
package app

import (
	db
)

Scope / {
	load user other.CurrentUser
	get /me Me.Show
}
