// spec/imports, two imports whose packages declare one name need an alias
// want: "db", which package lib__metrics declares, already used for package "lib__blog"
package app

import (
	lib__blog
	lib__metrics
)

scope / {
	resources db.posts
}
