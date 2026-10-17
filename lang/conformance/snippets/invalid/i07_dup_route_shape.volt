// spec: §route-conflicts
package app

Scope / {
	get /users/:id   Users.Show
	get /users/:name Users.ByName
}
