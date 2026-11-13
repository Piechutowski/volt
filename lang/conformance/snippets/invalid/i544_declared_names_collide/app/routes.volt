// spec: §imports, two imports whose packages declare one name need an alias
// want: import qualifier "db", which package lib__metrics declares, already used for package "lib__blog" (§imports); alias one of them
package app

import (
	lib__blog
	lib__metrics
)

scope / {
	resources db.posts
}
