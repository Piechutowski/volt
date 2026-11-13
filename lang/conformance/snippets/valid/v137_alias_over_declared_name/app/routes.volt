// spec: §imports, an alias overrides the name the imported package declares: lib__store declares db, imported here as store
package app

import (
	store lib__store
)

scope / {
	resources store.posts
}
