// spec: §V10.3 — a text column is not orderable: text > number is an error
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
}

Select bad for ms_revenue where org > 1
