// spec: §settings, a resources key parameter does not repeat a parameter of the enclosing scope
// want: rename it with [param:
package app

Table posts {
  id int [pk]
}

Scope /blogs/:id {
	resources posts
}
