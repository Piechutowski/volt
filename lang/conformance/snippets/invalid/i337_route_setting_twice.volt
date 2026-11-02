// spec: §settings-lists, §routes, a route takes each setting at most once
// want: setting "name" may appear at most once
package app

scope / {
	get /posts/:id(int) Posts.Show [name: post, name: entry]
}
