// spec: §V10.3 — like needs a text column
package db

Table ms_revenue {
  id integer [pk]
  year integer [not null]
}

Select bad for ms_revenue where year like 'x%'
