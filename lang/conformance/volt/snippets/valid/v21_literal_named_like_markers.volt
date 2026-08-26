// spec: §V4.7.1 — literals spelled P or W do not collide with the
// parameter and wildcard shape markers
package app

Scope / {
	get /P     A.LitP
	get /:q    A.ParQ
	get /W/x   B.LitW
	get /:rr/x B.ParR
}
