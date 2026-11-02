// spec: §validators — a variadic parameter never matches
// want: is variadic
package db

table users {
  id int [pk]
  email string

  validators {
    EmailValid(email)
  }
}
