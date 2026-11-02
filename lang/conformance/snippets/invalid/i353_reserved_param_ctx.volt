// spec: §route-paths, ctx names the context of every generated client method, so a path parameter does not take it
// want: path parameter "ctx" collides with a name in generated signatures
package app

Scope / {
	get /x/:ctx Home.X
}
