// spec: §imports — an aliased import
package app

import (
	d shared/db
)

Scope / {
	resources d.posts
}
