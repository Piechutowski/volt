// spec: §V12.5 — one parameter per argument column: a two-parameter
// function cannot be called with one column
// want: takes 2 parameter(s)
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
