package db

Table users {
  id    int [pk, increment]
  email string    [unique]
}

Select picked for users where id in :ids
