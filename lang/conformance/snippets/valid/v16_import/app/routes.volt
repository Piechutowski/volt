// spec: §imports — importing a sibling package
package app

import (
	db
)

scope / {
	resources db.users
}
