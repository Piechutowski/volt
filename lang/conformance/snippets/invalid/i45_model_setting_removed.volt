// spec: §declaration
package app

table posts {
	id int [pk]
}

// model: is not a setting — the declaration names the model.
scope / {
	resources posts [model: Post]
}
