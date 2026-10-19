// spec: §the-action-table — the seven actions; update spans PATCH and PUT
package app

Table users {
	id int [pk]
}

Scope / {
	resources users
}
