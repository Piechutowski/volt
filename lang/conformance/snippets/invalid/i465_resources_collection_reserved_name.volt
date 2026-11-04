// spec: §generated-client, §scopes, a resources collection route that derives a name of the embedded volt.Client is renamed only by a scope name, which the error names
// want: client method "Stream" would take the name of the embedded volt.Client's method Stream (§generated-client); rename the route with a scope name
package app

table stream {
	id    int    [pk]
	title string
}

scope / {
	resources stream [singular: stream_item]
}
