// spec: §validation-checks — the parameter's spelled type must equal the column's
// generated Go type: an integer column cannot feed a string parameter
// want: parameter 1 of EmailValid is string
package db

Table users {
  id int [pk]
  email int [not null]

  checks {
    EmailValid(email)
  }
}
