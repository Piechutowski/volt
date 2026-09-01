// spec: §V10.3 — one parameter name, one type
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Select bad for ms_revenue where org = :v and year = :v
