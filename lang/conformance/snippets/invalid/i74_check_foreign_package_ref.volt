// spec: §V12.5 — a Go-reference check names a function of the
// containing package; imported packages need a local wrapper
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    other.EmailValid(email)
  }
}
