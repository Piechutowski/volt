// spec: §pipelines — a plug is func(http.Handler) http.Handler, spelled exactly
// want: is not middleware
package app

pipeline api {
	use BearerAuth
}

scope / [pipe: api] {
	get / Home.Index
}
