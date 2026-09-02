// spec: §V12.5 — the parameter's spelled type must equal the column's
// generated Go type: an integer column cannot feed a string parameter
// want: parameter 1 of EmailValid is string
package db

Table users {
  id integer [pk]
  email integer [not null]

  checks {
    EmailValid(email)
  }
}
