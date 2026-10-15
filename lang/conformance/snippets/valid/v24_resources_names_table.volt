// spec: §declaration
package app

Table posty [model: 'Post'] {
	id integer [pk]
}

// The declaration spells the table exactly as declared; the URL keeps
// that name and the member helper takes the model's (PathPost).
Scope / {
	resources posty
}
