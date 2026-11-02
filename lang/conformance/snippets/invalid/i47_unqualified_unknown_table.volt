// spec: §declaration
package app

table posts {
	id int [pk]
}

// No package prefix does not soften the rule: "ghosts" is declared
// nowhere, so the resource is an error, not a schemaless guess.
scope / {
	resources ghosts
}
