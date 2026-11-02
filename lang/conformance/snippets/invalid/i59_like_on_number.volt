// spec: §predicates — like needs a text column
package db

table ms_revenue {
  id int [pk]
  year int
}

select bad for ms_revenue where year like "x%"
