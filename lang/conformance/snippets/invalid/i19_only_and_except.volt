// spec: §settings
package app

Table users {
	id integer [pk]
}

Scope / {
	resources users [only: (index), except: (show)]
}
