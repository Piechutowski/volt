package db

Table users {
  id    int    [pk, increment]
  email string [unique]
}
