// spec: §validators — one parameter per argument column: a two-parameter
// function cannot be called with one column
// want: takes 2 parameter(s)
package db

Table users {
  id int [pk]
  email string [not null]

  validators {
    EmailValid(email)
  }
}
