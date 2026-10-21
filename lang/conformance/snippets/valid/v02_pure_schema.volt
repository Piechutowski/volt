// spec: §layers-files-and-the-superset-rule — the layering rule: a Part I file body is a valid Volt file body
package db

Table users {
	id    int [pk, increment]
	email string    [not null, unique]
}

Enum status {
	active
	blocked
}
