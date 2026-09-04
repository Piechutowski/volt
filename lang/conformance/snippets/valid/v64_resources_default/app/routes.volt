// spec: §V5.5 — resources [default]: every surviving action is a query route over the table's CRUD; implies api; except: takes one back
package app

import (
	db
)

Scope /api [name: api] {
	resources db.users [default]
	resources db.tags  [default, except: (delete)]

	delete /tags/:id(int32) Tags.Purge
}
