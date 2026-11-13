// spec: §projects-and-packages, §imports, a package's name need not be its directory's: lib__api declares api, its Go files agree, and the import lib__api/lib__db is qualified by the name db its package declares
package api

import (
	lib__api/lib__db
)

pipeline guarded {
	use BearerAuth
}

scope /api [pipe: guarded] {
	get  /customers         db.CustomerList
	get  /customers/:id(int) db.CustomerGet
	post /customers         db.CustomerCreate
}
