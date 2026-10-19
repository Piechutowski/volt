// spec: §validation-checks — a Go-reference check names a function of the
// containing package; imported packages need a local wrapper
package db

Table users {
  id int [pk]
  email string [not null]

  checks {
    other.EmailValid(email)
  }
}
