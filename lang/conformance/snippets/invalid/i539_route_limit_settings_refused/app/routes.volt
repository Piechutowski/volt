// spec: §settings-whitelists, §query-routes, a route takes no default_limit: or max_limit:; a handler that wants a default or a most for a select's limit: decides it and calls the select
// want: setting "default_limit" is not valid on a route (§settings-whitelists); valid: name, timeout, body, only
package app

import (
	db
)

scope /api {
	get /feed db.PostTop [default_limit: 10, max_limit: 50]
	get /home Pages.Home [name: home, max_limit: 20] {
		rows db.PostTop(n)
	}
}
