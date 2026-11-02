// spec: §route-paths
package app

scope / {
	get /users/:id( int32 ) Users.Show
}
