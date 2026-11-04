// spec: §route-loads, a list key one load requires and another takes optional is an error, a missing key being an empty list to one and absent to the other
// want: load b binds the list key "q" optional, and load a binds it required
package app

import (
	db
)

scope / {
	get /mixed Pages.Mixed {
		a db.PostAllOf(q)
		b db.PostAnyOf(q)
	}
}
