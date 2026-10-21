package db

Table users {
  id    int [pk, increment]
  email string    [unique]
}

Table marks {
  id int [pk]
}
