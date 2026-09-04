// spec: §V5.5 — i114_resources_default_unqualified
// want: qualify the table
package app

import (
	db
)

Scope /api {
	resources users [default]
}
