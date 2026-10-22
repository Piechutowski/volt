// spec: §route-conflicts, the root path satisfies a wildcard at the root, and a root route, matching that path alone, is more specific than the wildcard, so the routes register and ServeMux picks the root route for /
package app

Scope / {
	get  /         Home.Index
	get  /:rest... Home.Any
	head /         Home.Probe
}
