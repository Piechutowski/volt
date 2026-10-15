// spec: §handler-references rule 3 — same action, identical signatures; §route-names-and-reverse-urls rule 2 requires
// distinct helper names, provided here by [name:]
package app

Scope / {
	get  /users/:id(int64)  Users.Show
	post /users/:id(int64)  Users.Show [name: showSubmit]
}
