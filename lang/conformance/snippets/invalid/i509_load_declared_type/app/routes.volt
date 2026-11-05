// spec: §route-loads, a parameter of a declared type is spelled by neither the query string, a path parameter nor a literal, so no load binds it
// want: parameter "price" of db.ProductCosting is money.Money, which no load binds
package app

import (
	db
)

scope / {
	get /costing Pages.Costing [name: costing] {
		rows db.ProductCosting(price)
	}
}
