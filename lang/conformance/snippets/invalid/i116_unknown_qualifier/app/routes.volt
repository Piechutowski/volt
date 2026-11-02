// spec: §imports, a qualifier names an import or the runtime: one that names neither, in a plug, a resources, a dataset or a handler, is an error
// want: unknown package qualifier "store"
package app

pipeline api {
	use store.Auth
}

scope /api [pipe: api] {
	resources store.users [default]
	dataset store.browse
	get /x store.UserList
}
