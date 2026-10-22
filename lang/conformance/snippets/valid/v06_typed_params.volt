// spec: §route-paths, the closed parameter type set: :name(int) is an int, a bare :name a string
package app

Scope / {
	get /a/:x(int) A.X
	get /b/:y(int) B.Y
	get /c/:z(int) C.Z
	get /d/:s      D.S
	get /e/:t      E.T
}
