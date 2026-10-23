// spec: §settings-lists, §scopes, a Scope takes each setting at most once
// want: setting "pipe" may appear at most once
package app

Pipeline api {
	use volt.RequestID
}

Scope /x [pipe: api, pipe: api] {
	get /ping Home.Ping
}
