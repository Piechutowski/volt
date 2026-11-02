// spec: §settings-lists, §scopes, a Scope's settings list stays on the line of the Scope
// want: a settings list stays on the line of the scope it modifies
package app

pipeline api {
	use volt.RequestID
}

scope /
	[pipe: api] {
	get /ping Home.Ping
}
