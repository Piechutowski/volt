// spec: §pipelines rule 2, §route-paths rule 6
package app

Pipeline api {
	use "weird name"
}

Scope / [pipe: api] {
	get /x X.Y
}
