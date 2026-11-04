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

select browse for series where year = :year [order: (id asc)]
