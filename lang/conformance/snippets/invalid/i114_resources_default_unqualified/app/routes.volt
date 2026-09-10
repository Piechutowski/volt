// spec: §V5.1, §V5.5 — i114_resources_default_unqualified: a bare table is this package's own, which declares none; the imported package's must be qualified
// want: declares no tables
package app

import (
	db
)

Scope /api {
	resources users [default]
}
