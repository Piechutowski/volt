// spec: §route-conflicts rule 2
package app

Scope / {
	get /users/:id   Users.Show
	get /users/:name Users.ByName
}
