// spec: §keywords, §route-paths, a parameter type is a lowercase keyword: a wrong spelling is an error naming the right one, reported once
// want: write int, not INT
package app

scope / {
	get /posts/:id(INT) Post.Show
	get /tags/:id(Int)/:page(int) Tag.Find
}
