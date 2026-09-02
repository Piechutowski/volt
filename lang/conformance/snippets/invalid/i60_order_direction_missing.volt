// spec: §V11.5 — order directions are mandatory: no silent asc
package db

Table ms_revenue {
  id integer [pk]
  year integer [not null]
}

Select rows for ms_revenue where year >= :y [order: (year desc, id)]
