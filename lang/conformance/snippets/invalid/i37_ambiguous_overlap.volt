// spec: §V4.7.2
package app

// /a/b matches both, and neither pattern is more specific than the
// other — ServeMux would panic at registration; Volt rejects at check.
Scope / {
	get /a/:x  A.One
	get /:y/b  A.Two
}
