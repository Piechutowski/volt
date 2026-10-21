// spec: §table-resolution — model resolution within the package
package app

Table users {
	id int [pk, increment]
}

Scope / {
	resources users
}
