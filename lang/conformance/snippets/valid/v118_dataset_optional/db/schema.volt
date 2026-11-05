package db

table ms_revenue {
  id   int [pk]
  org  string
  year int
}

table ms_usage {
  id   int [pk]
  org  string
  year int
}

group series {
  ms_revenue
  ms_usage
}

select browse for series where year = :year? and org = :org [order: (id asc)]
select picked for series where id in :ids? [order: (id asc)]
