// spec: §declaration
package app

Table posts {
	id integer [pk]
}

// model: is not a setting — the declaration names the model.
Scope / {
	resources posts [model: Post]
}
