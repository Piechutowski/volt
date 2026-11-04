// spec: §route-loads, an optional parameter of a declared type binds from no query key and no literal, so either spelling is the one error
// want: parameter "price" of db.ProductCosting is rt.Null[money.Money], which no load binds
package app

import (
	db
)

scope / {
	get /costing Pages.Costing [name: costing] {
		rows db.ProductCosting(price)
	}
	get /cheap Pages.Cheap [name: cheap] {
		rows db.ProductCosting(1)
	}
}
