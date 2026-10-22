// spec: §validators — a Go-reference check returns exactly error
// want: must return exactly error
package db

Table users {
  id int [pk]
  email string

  Validators {
    EmailValid(email)
  }
}
