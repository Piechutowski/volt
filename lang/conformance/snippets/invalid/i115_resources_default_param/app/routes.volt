// spec: §V5.5 — i115_resources_default_param
// want: param: does not apply with [default]
package app

import (
	db
)

Scope /api {
	resources db.users [default, param: key]
}
