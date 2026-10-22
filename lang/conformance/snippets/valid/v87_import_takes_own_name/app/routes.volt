// spec: §imports, an import may take the package's own name as its qualifier, since a package names its own declarations bare and its name qualifies nothing else
package app

import (
	app db
)

Scope /api {
	resources app.users [default]
}
