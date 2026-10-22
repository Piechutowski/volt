// spec: §validators — a Go-reference check names a function that must exist
// in the package's Go files (a typo is an error here, not in generated code)
// want: no function EmailValdi
package db

Table users {
  id int [pk]
  email string

  Validators {
    EmailValdi(email)
  }
}
