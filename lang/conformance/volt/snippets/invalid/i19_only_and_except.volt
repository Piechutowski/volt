// spec: §V5.3.1
package app

Table users {
	id integer [pk]
}

Scope / {
	resources users [only: (index), except: (show)]
}
