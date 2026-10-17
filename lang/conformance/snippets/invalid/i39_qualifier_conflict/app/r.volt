// spec: §imports
package app

// Two different paths under one qualifier: 'db' names both db and
// backup/db.
import (
	db
	db backup/db
)

Scope / {
	resources db.users
}
