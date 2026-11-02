package db

Table users {
  id    int    [pk, increment]
  email string [unique]
}

Table metrics {
  id int [pk, increment]
}

Select ordered for metrics
