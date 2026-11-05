// spec: §query-routes, an optional parameter of a declared type binds from nowhere either: its rt.Null is no type the query string carries
// want: parameter "price" of db.ProductCosting is rt.Null[money.Money], which the query string cannot carry
package app

import (
	db
)

scope /api {
	get /costing db.ProductCosting
}
