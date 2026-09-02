// spec: §V12.5 — a generic function cannot be referenced: the generated
// call could not instantiate it
// want: is generic
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    EmailValid(email)
  }
}
