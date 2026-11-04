// spec: §route-names-and-reverse-urls, §settings, a resources member helper that collides with an earlier route's is renamed by singular:, which the error names, since resources take no name:
// want: (§route-names-and-reverse-urls, §generated-client); disambiguate with [singular:] or a scope name
package app

table posts {
	id    int    [pk]
	title string
}

scope / {
	get /lead Home.Lead [name: post]
	resources posts
}
