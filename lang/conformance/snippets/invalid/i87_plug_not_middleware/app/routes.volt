// spec: §V3.2 — a plug is func(http.Handler) http.Handler, spelled exactly
// want: is not middleware
package app

Pipeline api {
	use BearerAuth
}

Scope / [pipe: api] {
	get / Home.Index
}
