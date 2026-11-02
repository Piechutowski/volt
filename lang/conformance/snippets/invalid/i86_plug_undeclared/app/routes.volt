// spec: §pipelines — a plug of this package must exist in its Go files
// want: no function BearerAuth
package app

pipeline api {
	use BearerAuth
}

scope / [pipe: api] {
	get / Home.Index
}
