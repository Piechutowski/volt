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

select recent for series where year = :year? [order: (id desc), limit: :limit]

table posts {
  id int [pk]
}

select top for posts [order: (id desc), limit: :limit]
