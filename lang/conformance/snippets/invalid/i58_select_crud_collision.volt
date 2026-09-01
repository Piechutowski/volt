// spec: §V11.1 — a select may not shadow the generated CRUD surface
package db

Table ms_revenue {
  id integer [pk]
  year integer [not null]
}

Select list for ms_revenue where year = :y
