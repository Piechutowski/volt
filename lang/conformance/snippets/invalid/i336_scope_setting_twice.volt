// spec: §settings-lists, §scopes, a Scope takes each setting at most once
// want: setting "pipe" may appear at most once
package app

pipeline api {
	use volt.RequestID
}

scope /x [pipe: api, pipe: api] {
	get /ping Home.Ping
}
