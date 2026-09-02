// spec: §V5.1
package app

Table posts {
	id integer [pk]
}

// No package prefix does not soften the rule: "ghosts" is declared
// nowhere, so the resource is an error, not a schemaless guess.
Scope / {
	resources ghosts
}
