// spec: §predicates — a text column is not orderable: text > number is an error
package db

table ms_revenue {
  id int [pk]
  org string
}

select bad for ms_revenue where org > 1
