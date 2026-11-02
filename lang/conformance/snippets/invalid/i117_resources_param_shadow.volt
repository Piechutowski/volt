// spec: §settings, a resources key parameter does not repeat a parameter of the enclosing scope
// want: rename it with [param:
package app

table posts {
  id int [pk]
}

scope /blogs/:id {
	resources posts
}
