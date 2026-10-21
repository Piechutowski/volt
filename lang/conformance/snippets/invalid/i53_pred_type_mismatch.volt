// spec: §predicates — a text column is not orderable: text > number is an error
package db

Table ms_revenue {
  id int [pk]
  org string
}

Select bad for ms_revenue where org > 1
