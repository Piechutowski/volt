// spec: §scope-loads — a nested scope cannot load a name again
// want: is already loaded
package app

import (
	db
)

scope / {
	load user CurrentUser
	scope /admin {
		load user Tenant
		get /stats Admin.Stats
	}
}
