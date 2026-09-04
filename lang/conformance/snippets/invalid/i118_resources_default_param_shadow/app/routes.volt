// spec: §V5.5 — i118_resources_default_param_shadow
// want: give the scope's parameter another name
package app

import (
	db
)

Scope /farms/:id(string) {
	resources db.plots [default]
}
