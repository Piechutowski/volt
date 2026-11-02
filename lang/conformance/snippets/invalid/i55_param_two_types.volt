// spec: §predicates — one parameter name, one type
package db

table ms_revenue {
  id int [pk]
  org string
  year int
}

select bad for ms_revenue where org = :v and year = :v
