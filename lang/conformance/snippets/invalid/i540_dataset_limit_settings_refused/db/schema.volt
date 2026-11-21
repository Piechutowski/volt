package db

table ms_revenue {
  id   int [pk]
  year int
}

table ms_usage {
  id   int [pk]
  year int
}

group series {
  ms_revenue
  ms_usage
}

select recent for series [order: (id desc), limit: :limit]
