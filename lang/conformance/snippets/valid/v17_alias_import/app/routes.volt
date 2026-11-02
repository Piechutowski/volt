// spec: §imports — an aliased import
package app

import (
	d shared/db
)

scope / {
	resources d.posts
}
