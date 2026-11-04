// spec: §route-loads, max_limit: caps a select's limit: parameter a path parameter binds as well as one the query string binds, and either setting takes any positive integer that fits int64
package app

import (
	db
)

scope /api {
	get /top/:limit(int) db.PostFeed [max_limit: 50]
	get /home/:n(int) Pages.Home [name: home, max_limit: 20] {
		feed db.PostFeed(tag, at, id, n)
	}
	get /all db.PostFeed [name: all, limit: 2147483648, max_limit: 9223372036854775807]
}
