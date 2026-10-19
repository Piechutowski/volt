// spec: §validation-checks — only Go files the go tool would compile count: a
// declaration in a _-prefixed file does not exist for the package
// want: no function EmailValid
package db

Table users {
  id int [pk]
  email string [not null]

  checks {
    EmailValid(email)
  }
}
