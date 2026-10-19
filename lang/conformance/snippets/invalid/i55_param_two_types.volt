// spec: §predicates — one parameter name, one type
package db

Table ms_revenue {
  id int [pk]
  org string [not null]
  year int [not null]
}

Select bad for ms_revenue where org = :v and year = :v
