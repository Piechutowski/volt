// spec: §generated-client, §settings, a resources member route that derives a name of the embedded volt.Client is renamed by singular:, which the error names, since resources take no name:
// want: client method "Header" would take the name of the embedded volt.Client's field Header (§generated-client); rename the route with [singular:] or a scope name
package app

table headers {
	id    int    [pk]
	title string
}

scope / {
	resources headers
}
