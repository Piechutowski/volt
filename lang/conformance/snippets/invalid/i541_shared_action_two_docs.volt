// spec: §handler-references, §doc-comments-in-generated-code, of the routes that share an action at most one carries a doc comment, since the action's method has one
// want: is documented by the route at
package app

scope / {
	// The posts, newest first.
	get /posts Posts.Index [name: posts]
	// The posts under their old address.
	get /blog  Posts.Index [name: blog]
}
