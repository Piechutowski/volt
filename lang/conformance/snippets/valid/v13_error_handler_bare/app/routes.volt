// spec: §scopes, §imports, error_handler names a function of the package bare, scope by scope
package app

Scope /a [error_handler: Errors] {
	get /x X.A
}
Scope /b [error_handler: Errors] {
	get /x X.B
}
