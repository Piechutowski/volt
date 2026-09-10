// spec: §V2.4 — i116_import_own_name: the package's own name qualifies its own declarations, so an import cannot take it
// want: is this package's own name
package app

import (
	app db
)

Scope /api {
	resources app.users [default]
}
