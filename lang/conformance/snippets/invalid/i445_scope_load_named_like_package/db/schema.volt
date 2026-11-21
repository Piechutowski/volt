package db

table users {
  id    int    [pk]
  email string [unique]
}

table metrics {
  id int [pk]
}

select ordered for metrics
