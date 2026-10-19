package db

Table users {
  id    int [pk, increment]
  email string    [not null, unique]
}

Select picked for users where id in :ids
