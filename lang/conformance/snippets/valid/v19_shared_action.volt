// spec: §V4.3.3 — same action, identical signatures; §V4.6.2 requires
// distinct helper names, provided here by [name:]
package app

Scope / {
	get  /users/:id(int64)  Users.Show
	post /users/:id(int64)  Users.Show [name: showSubmit]
}
