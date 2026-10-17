// spec: §route-paths — the closed parameter type set
package app

Scope / {
	get /a/:x(int)    A.X
	get /b/:y(int32)  B.Y
	get /c/:z(int64)  C.Z
	get /d/:s(string) D.S
	get /e/:t         E.T
}
