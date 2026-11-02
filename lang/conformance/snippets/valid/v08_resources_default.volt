// spec: §the-action-table, the seven actions, one route each; update is PATCH alone
package app

table users {
	id int [pk]
}

scope / {
	resources users
}
