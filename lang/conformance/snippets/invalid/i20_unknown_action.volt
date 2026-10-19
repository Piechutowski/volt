// spec: §settings
package app

Table users {
	id int [pk]
}

Scope / {
	resources users [only: (browse)]
}
