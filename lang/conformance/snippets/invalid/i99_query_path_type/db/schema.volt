package db

table users {
  id    int [pk, increment]
  email string    [unique]
}

select picked for users where id in :ids
