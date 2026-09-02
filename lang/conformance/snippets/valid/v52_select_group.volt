// spec: §V11 — one select, one signature, every member; order setting
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Table ms_usage {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Group series {
  ms_revenue
  ms_usage
}

Pred current { org = :org and year = :year }

Select rows for series where current [order: (year desc, id asc)]
Select all for series
