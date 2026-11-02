// spec: §route-paths — ':name...' captures the rest of the path
package app

scope / {
	get /files/:path... Files.Serve
}
