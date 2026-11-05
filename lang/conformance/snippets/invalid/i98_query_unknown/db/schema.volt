package db

table users {
  id    int [pk]
  email string    [unique]
}

select picked for users where id in :ids
