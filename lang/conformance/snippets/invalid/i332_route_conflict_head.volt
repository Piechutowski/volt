// spec: §route-conflicts, §routes, ServeMux serves HEAD for a GET pattern, so a head route overlaps a get route: HEAD /a/b matches both of the first pair, and HEAD /c both of the second, while neither route of a pair is more specific
// want: route HEAD /:y/b is ambiguous with the route at
package app

Scope / {
	get  /a/:x A.One
	head /:y/b A.Two
	head /:z   A.Three
	get  /c    A.Four
}
