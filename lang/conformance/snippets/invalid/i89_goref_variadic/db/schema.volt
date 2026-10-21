// spec: §validators — a variadic parameter never matches
// want: is variadic
package db

Table users {
  id int [pk]
  email string

  validators {
    EmailValid(email)
  }
}
