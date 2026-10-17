// spec: §pipelines, §route-paths
package app

Pipeline api {
	use "weird name"
}

Scope / [pipe: api] {
	get /x X.Y
}
