// spec: §settings-lists, §scopes, a Scope's settings list stays on the line of the Scope
// want: a settings list stays on the line of the Scope it modifies
package app

Pipeline api {
	use volt.RequestID
}

Scope /
	[pipe: api] {
	get /ping Home.Ping
}
