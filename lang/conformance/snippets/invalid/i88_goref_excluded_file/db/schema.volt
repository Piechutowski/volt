// spec: §V12.5 — only Go files the go tool would compile count: a
// declaration in a _-prefixed file does not exist for the package
// want: no function EmailValid
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
