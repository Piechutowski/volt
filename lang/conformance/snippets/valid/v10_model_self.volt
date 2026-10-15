// spec: §table-resolution — model resolution within the package
package app

Table users {
	id integer [pk, increment]
}

Scope / {
	resources users
}
