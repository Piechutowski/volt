package db

table users {
  id    int    [pk]
  email string [unique]
}
