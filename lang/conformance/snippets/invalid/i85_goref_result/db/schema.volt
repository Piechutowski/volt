// spec: §validators — a Go-reference check returns exactly error
// want: must return exactly error
package db

table users {
  id int [pk]
  email string

  validators {
    EmailValid(email)
  }
}
