// spec: §V12.5 — a variadic parameter never matches
// want: is variadic
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
