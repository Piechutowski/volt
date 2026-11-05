package db

table users {
  id    int [pk]
  email string    [unique]
}

table marks {
  id int [pk]
}
