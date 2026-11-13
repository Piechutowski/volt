// Package api is the shop's API, in the directory lib__api: its name is
// the one its package clause declares (§projects-and-packages).
package api

import (
	lib__api/lib__db
	stats lib__stats
)

pipeline guarded {
	use BearerAuth
}

scope /api [pipe: guarded] {
	get  /customers                db.CustomerList
	get  /customers/:id(int)       db.CustomerGet
	post /customers                db.CustomerCreate
	get  /customers/:id(int)/card  Customers.Card {
		customer db.CustomerGet(id)
	}
	resources stats.samples [default, only: (index, show)]
}
