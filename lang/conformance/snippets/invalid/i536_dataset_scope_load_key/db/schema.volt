package db

table users {
  id    int    [pk]
  email string [unique]
}

table ms_revenue {
  id   int [pk]
  user int
}

table ms_usage {
  id   int [pk]
  user int
}

group series {
  ms_revenue
  ms_usage
}

select recent for series where user = :user [order: (id desc)]
