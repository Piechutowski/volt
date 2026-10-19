// spec: §validators — the function's parameter type disagrees with the
// text column it is passed
// want: parameter 1 of EmailValid is int
package db

Table users {
  id int [pk]
  email string [not null]

  validators {
    EmailValid(email)
  }
}
