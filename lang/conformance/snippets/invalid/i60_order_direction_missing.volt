// spec: §selects-over-groups — order directions are mandatory: no silent asc
package db

Table ms_revenue {
  id int [pk]
  year int
}

Select rows for ms_revenue where year >= :y [order: (year desc, id)]
