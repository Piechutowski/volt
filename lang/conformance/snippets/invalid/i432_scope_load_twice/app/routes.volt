// spec: §scope-loads — a nested scope cannot load a name again
// want: is already loaded
package app

import (
	db
)

Scope / {
	load user CurrentUser
	Scope /admin {
		load user Tenant
		get /stats Admin.Stats
	}
}
