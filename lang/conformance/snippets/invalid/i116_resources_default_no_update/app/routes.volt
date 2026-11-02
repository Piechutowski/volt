// spec: §default-handlers — i116_resources_default_no_update
// want: no update method
package app

import (
	db
)

scope /api {
	resources db.marks [default]
}
