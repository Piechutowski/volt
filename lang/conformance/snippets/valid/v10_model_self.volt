// spec: §V5.4 — model resolution within the package
package app

Table users {
	id integer [pk, increment]
}

Scope / {
	resources users [model: User]
}
