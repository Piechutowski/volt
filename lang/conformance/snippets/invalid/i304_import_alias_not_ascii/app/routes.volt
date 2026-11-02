// spec: §imports, an import alias is an identifier of ASCII letters, digits and underscores, as the package path's segments are
// want: import alias "ä" is not an identifier of ASCII letters, digits and underscores
package app

import (
	ä db
)

scope /api {
	get /users ä.UserList
}
