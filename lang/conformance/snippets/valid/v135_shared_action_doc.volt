// spec: §handler-references, §doc-comments-in-generated-code, one of the routes that share an action documents the action's method

package app

scope / {
	// The posts, newest first.
	get /posts Posts.Index [name: posts]
	get /blog  Posts.Index [name: blog]
}
