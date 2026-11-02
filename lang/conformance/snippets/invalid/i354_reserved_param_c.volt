// spec: §route-paths, c is the receiver of every generated client method, so a path parameter does not take it
// want: path parameter "c" collides with a name in generated signatures
package app

Scope / {
	get /y/:c Home.Y
}
