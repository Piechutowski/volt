// spec: §selects-over-groups — one select, one signature, every member; order setting
package db

table ms_revenue {
  id int [pk]
  org string
  year int
}

table ms_usage {
  id int [pk]
  org string
  year int
}

group series {
  ms_revenue
  ms_usage
}

pred current { org = :org and year = :year }

select rows for series where current [order: (year desc, id asc)]
select all for series
