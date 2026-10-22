// spec: §predicates — like needs a text column
package db

Table ms_revenue {
  id int [pk]
  year int
}

Select bad for ms_revenue where year like "x%"
