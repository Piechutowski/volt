package db

table users {
  id    int    [pk, increment]
  email string [unique]
}
