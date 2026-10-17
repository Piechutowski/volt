// spec: §layers-files-and-the-superset-rule — the superset rule: DBML bodies are valid Volt
package db

Table users {
	id    integer [pk, increment]
	email text    [not null, unique]
}

Enum status {
	active
	blocked
}
