// spec: §validators — a generic function cannot be referenced: the generated
// call could not instantiate it
// want: is generic
package db

table users {
  id int [pk]
  email string

  validators {
    EmailValid(email)
  }
}
