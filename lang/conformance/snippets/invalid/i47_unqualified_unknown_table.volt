// spec: §declaration
package app

Table posts {
	id int [pk]
}

// No package prefix does not soften the rule: "ghosts" is declared
// nowhere, so the resource is an error, not a schemaless guess.
Scope / {
	resources ghosts
}
