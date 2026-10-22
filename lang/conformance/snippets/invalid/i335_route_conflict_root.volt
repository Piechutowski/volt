// spec: §route-conflicts, the root path satisfies a wildcard at the root, so a head wildcard there overlaps a get root route: HEAD / matches both, and neither is more specific
// want: route HEAD /:rest... is ambiguous with the route at
package app

Scope / {
	get  /         Home.Index
	head /:rest... Files.Probe
}
