// spec: §layers-files-and-the-superset-rule — the superset rule: DBML bodies are valid Volt
package db

Table users {
	id    int [pk, increment]
	email string    [not null, unique]
}

Enum status {
	active
	blocked
}
