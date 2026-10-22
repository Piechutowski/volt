// spec: §the-action-table, the seven actions, one route each; update is PATCH alone
package app

Table users {
	id int [pk]
}

Scope / {
	resources users
}
