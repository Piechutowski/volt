// spec: §selects-over-groups — order directions are mandatory: no silent asc
package db

table ms_revenue {
  id int [pk]
  year int
}

select rows for ms_revenue where year >= :y [order: (year desc, id)]
