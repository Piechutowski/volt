// spec: §selects-over-groups — a select may not shadow the generated CRUD surface
package db

Table ms_revenue {
  id int [pk]
  year int
}

Select list for ms_revenue where year = :y
