// spec: §V5.2 — the seven actions; update spans PATCH and PUT
package app

Table users {
	id integer [pk]
}

Scope / {
	resources users
}
