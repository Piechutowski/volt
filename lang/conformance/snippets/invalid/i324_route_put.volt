// spec: §routes, put is not a route verb: the partial update is patch, and a put line on a controller route, the word in any case, is an error naming it, once per line
// want: put is not a route verb: the partial update is patch, so write patch /posts/:id(int) Posts.Replace (§routes)
package app

Scope / {
	patch /posts/:id(int) Posts.Update
	put   /posts/:id(int) Posts.Replace
	PUT   /tags/:slug Tags.Replace
}
