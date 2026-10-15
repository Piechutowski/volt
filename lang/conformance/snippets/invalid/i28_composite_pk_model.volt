// spec: §table-resolution rule 3
package app

Table pairs {
	a integer
	b integer
	Indexes { (a, b) [pk] }
}

Scope / {
	resources pairs
}
