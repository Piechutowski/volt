// spec: §V2.3 — an aliased import
package app

import (
	d shared/db
)

Scope / {
	resources d.posts
}
