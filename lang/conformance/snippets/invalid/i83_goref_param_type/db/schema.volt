// spec: §V12.5 — the function's parameter type disagrees with the
// text column it is passed
// want: parameter 1 of EmailValid is int
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
