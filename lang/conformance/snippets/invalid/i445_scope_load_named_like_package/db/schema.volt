package db

table users {
  id    int    [pk, increment]
  email string [unique]
}

table metrics {
  id int [pk, increment]
}

select ordered for metrics
