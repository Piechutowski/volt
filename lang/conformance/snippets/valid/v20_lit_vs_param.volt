// spec: §route-conflicts — literal vs parameter is not a conflict
package app

scope / {
	get /users/new Users.New
	get /users/:id Users.Show
}
