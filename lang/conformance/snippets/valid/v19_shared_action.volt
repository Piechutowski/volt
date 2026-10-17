// spec: §handler-references — same action, identical signatures; §route-names-and-reverse-urls requires
// distinct helper names, provided here by [name:]
package app

Scope / {
	get  /users/:id(int64)  Users.Show
	post /users/:id(int64)  Users.Show [name: showSubmit]
}
