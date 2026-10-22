// spec: §default-handlers — i118_resources_default_param_shadow
// want: give the scope's parameter another name
package app

import (
	db
)

Scope /farms/:id {
	resources db.plots [default]
}
