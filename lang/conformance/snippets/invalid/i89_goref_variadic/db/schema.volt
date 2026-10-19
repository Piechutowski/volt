// spec: §validation-checks — a variadic parameter never matches
// want: is variadic
package db

Table users {
  id int [pk]
  email string [not null]

  checks {
    EmailValid(email)
  }
}
