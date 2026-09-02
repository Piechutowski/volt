// spec: §V12.5 — a Go-reference check returns exactly error
// want: must return exactly error
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
