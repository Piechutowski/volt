// spec: §table-resolution
package app

table pairs {
	a int [pk]
	b int [pk]
}

scope / {
	resources pairs
}
