// spec: §V5.1
package app

Table posty [model: 'Post'] {
	id integer [pk]
}

// The declaration names the model, so the URL keeps the table's name
// while the member helper uses the model's — no singularization guess.
Scope / {
	resources Post
}
