// spec: §layers-files-and-the-superset-rule, the layering rule: a Part I file body is a valid Volt file body
package db

table users {
	id    int [pk, increment]
	email string    [unique]
}

type Status string
