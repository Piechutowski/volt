// spec: §route-conflicts rule 2 — literal vs parameter is not a conflict
package app

Scope / {
	get /users/new Users.New
	get /users/:id Users.Show
}
