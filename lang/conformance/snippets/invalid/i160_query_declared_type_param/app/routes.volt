// spec: §query-routes, a parameter of a declared type binds from nowhere: the select is legal, the route is not
// want: which the query string cannot carry
package app

import (
	db
)

Scope /api {
	get /costing db.ProductCosting
}
