// spec: §default-handlers, under [default] the key parameter is the generated CRUD's, so the scope's parameter is the one to rename
// want: give the scope's parameter another name
package app

import (
	db
)

Scope /blogs/:id {
	resources db.posts [default]
}
