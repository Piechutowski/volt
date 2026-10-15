// spec: §default-handlers — i116_resources_default_no_update
// want: no update method
package app

import (
	db
)

Scope /api {
	resources db.marks [default]
}
