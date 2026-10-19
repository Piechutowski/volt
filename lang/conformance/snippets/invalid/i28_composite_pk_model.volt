// spec: §table-resolution
package app

Table pairs {
	a int
	b int
	Indexes { (a, b) [pk] }
}

Scope / {
	resources pairs
}
