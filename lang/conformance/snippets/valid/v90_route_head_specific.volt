// spec: §route-conflicts, §routes, a head route on a path a get route also matches is more specific than it, since its method and its paths are subsets of the get route's, so the pair registers and ServeMux picks the head route for HEAD
package app

Scope / {
	get  /:slug Pages.Show
	head /about Pages.Probe
	get  /posts Posts.Index
	head /posts Posts.Peek
}
