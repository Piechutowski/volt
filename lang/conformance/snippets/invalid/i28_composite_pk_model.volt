// spec: §table-resolution
package app

Table pairs {
	a integer
	b integer
	Indexes { (a, b) [pk] }
}

Scope / {
	resources pairs
}
