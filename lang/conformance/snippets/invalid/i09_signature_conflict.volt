// spec: §handler-references — one handler, two typed signatures for the same parameter
package app

Scope / {
	get /a/:id(int) X.Show [name: showA]
	get /b/:id X.Show [name: showB]
}
