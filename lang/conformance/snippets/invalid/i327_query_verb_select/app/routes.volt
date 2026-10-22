// spec: §query-routes, a select method is routed with get, as List is; options and any are errors naming get
// want: db.PostPopular is routed with get, not options
package app

import (
	db
)

Scope /api {
	options /popular db.PostPopular
	any     /hot     db.PostPopular [name: hot]
}
