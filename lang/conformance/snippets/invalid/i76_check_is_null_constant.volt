// spec: §V12.3 — is null is constant in a check: its columns are not
// null by rule
package db

Table users {
  id integer [pk]
  email varchar [not null]

  checks {
    email is not null
  }
}
