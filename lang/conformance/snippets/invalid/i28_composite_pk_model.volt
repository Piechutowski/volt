// spec: §table-resolution
package app

Table pairs {
	a int [pk]
	b int [pk]
}

Scope / {
	resources pairs
}
