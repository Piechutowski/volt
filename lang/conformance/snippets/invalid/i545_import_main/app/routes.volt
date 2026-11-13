// spec: §imports, a package declaring main is a program, which Go cannot import
// want: package "tool" declares main, a program, which Go cannot import (§imports)
package app

import (
	tool
)

scope / {
	resources main.posts
}
