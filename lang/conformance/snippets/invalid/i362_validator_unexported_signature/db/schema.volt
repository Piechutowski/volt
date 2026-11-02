// spec: §validators, an unexported validator is an error naming the exact signature to write
// want: so declare func EmailShape(string) error
package db

Table users {
  id    int    [pk]
  email string

  Validators {
    emailShape(email)
  }
}
