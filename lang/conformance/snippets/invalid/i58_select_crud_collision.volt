// spec: §selects-over-groups — a select may not shadow the generated CRUD surface
package db

table ms_revenue {
  id int [pk]
  year int
}

select list for ms_revenue where year = :y
