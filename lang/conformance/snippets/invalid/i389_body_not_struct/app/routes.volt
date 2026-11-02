// spec: §request-bodies — a body decodes into a struct type
// want: is int, not a struct type
package app

scope / {
	post /clock Session.Clock [body: Clock]
}
