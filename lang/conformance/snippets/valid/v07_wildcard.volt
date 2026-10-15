// spec: §route-paths rule 4 — ':name...' captures the rest of the path
package app

Scope / {
	get /files/:path... Files.Serve
}
