// spec: §route-loads, default_limit: over a limit whose key another load requires governs nothing, a request without the key being refused before the default applies
// want: default_limit: is the limit a request that leaves the key out gets, and load more requires the key "n"
package app

import (
	db
)

scope /api {
	get /s Pages.S [default_limit: 2] {
		rows db.PostTop(n)
		more db.PostByViews(n)
	}
}
