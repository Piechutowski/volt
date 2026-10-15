// spec: §imports rule 3 — an aliased import
package app

import (
	d shared/db
)

Scope / {
	resources d.posts
}
