// spec: §selects-over-groups — one select, one signature, every member; order setting
package db

Table ms_revenue {
  id int [pk]
  org string
  year int
}

Table ms_usage {
  id int [pk]
  org string
  year int
}

Group series {
  ms_revenue
  ms_usage
}

Pred current { org = :org and year = :year }

Select rows for series where current [order: (year desc, id asc)]
Select all for series
