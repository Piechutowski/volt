// spec: §V3.2 — a plug of this package must exist in its Go files
// want: no function BearerAuth
package app

Pipeline api {
	use BearerAuth
}

Scope / [pipe: api] {
	get / Home.Index
}
