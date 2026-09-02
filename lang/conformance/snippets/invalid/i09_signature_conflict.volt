// spec: §V4.3.3 — one handler, two typed signatures for the same parameter
package app

Scope / {
	get /a/:id(int64) X.Show [name: showA]
	get /b/:id(int32) X.Show [name: showB]
}
