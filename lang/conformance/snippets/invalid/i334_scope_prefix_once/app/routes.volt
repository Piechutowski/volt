// spec: §route-paths, §scopes, a Scope prefix is checked once where it is written: an unknown type, a reserved name or a duplicate in it is one error however many routes, resources actions or none it holds, and a route parameter repeating a prefix parameter is reported at the route
// want: unknown parameter type "uuid" (§route-paths); valid: int
package app

import (
	db
)

Scope /orgs/:org(uuid) {
	get /a Orgs.A
	get /b Orgs.B
	resources db.tags
}

Scope /w/:w {}

Scope /d/:x/:x {
	get /a Dups.A
}

Scope /n/:n {
	get /m/:n Nests.M
}
