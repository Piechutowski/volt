// spec: §table-resolution — model resolution within the package
package app

table users {
	id int [pk, increment]
}

scope / {
	resources users
}
