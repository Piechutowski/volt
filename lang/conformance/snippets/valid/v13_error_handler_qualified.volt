// spec: §V4.4 — error_handler as Name or <package>.Name
package app

Scope /a [error_handler: Errors] {
	get /x X.A
}
Scope /b [error_handler: app.Errors] {
	get /x X.B
}
