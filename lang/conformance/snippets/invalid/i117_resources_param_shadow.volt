// spec: §V5.3 — i117_resources_param_shadow
// want: rename it with [param:
package app

Table plots {
  id integer [pk]
}

Scope /farms/:id(string) {
	resources plots
}
