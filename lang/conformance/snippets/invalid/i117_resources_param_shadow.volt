// spec: §settings — i117_resources_param_shadow
// want: rename it with [param:
package app

Table plots {
  id int [pk]
}

Scope /farms/:id(string) {
	resources plots
}
