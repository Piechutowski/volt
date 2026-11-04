// spec: §imports, a qualifier is none of the names the route files and the client import for themselves, by default or by alias, and the error names an alias to write
// want: import qualifier "rt" is the name the generated route files and client import the data runtime nao/rt under (§imports); alias the import: rtdb rt
package app

import (
	rt
	run db
)

scope /api {
	get /feed  rt.PostFeed
	get /items run.ItemList
}
