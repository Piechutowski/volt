// spec: §generated-client, a client method does not take a name of the embedded volt.Client
// want: client method "Raw" would take the name of the embedded volt.Client's method Raw
package app

Scope / {
	get /raw Home.Raw
	get /client Home.Client
}
