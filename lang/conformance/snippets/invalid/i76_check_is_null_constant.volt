// spec: §validation-checks — is null is constant in a check: its columns are not
// null by rule
package db

Table users {
  id int [pk]
  email string [not null]

  checks {
    email is not null
  }
}
