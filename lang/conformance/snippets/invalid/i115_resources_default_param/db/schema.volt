package db

Table users {
  id    int [pk, increment]
  email string    [not null, unique]
}

Table marks {
  id int [pk]
}
