// spec: §scope-loads — a scope load is func(volt.Request) volt.Load[T]
// want: is not a scope load
package app

import (
	db
)

Scope / {
	load user Plain
	get /me Me.Show
}
