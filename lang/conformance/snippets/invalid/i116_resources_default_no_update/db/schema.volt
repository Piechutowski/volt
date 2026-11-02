package db

table users {
  id    int [pk, increment]
  email string    [unique]
}

table marks {
  id int [pk]
}
