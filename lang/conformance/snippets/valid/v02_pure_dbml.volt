// spec: §V0.2 — the superset rule: EDBML bodies are valid Volt
package db

Table users {
	id    integer [pk, increment]
	email text    [not null, unique]
}

Enum status {
	active
	blocked
}
