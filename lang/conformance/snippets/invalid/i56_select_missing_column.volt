// spec: §selects-over-groups — every member must have every referenced column
package db

table ms_revenue {
  id int [pk]
  org string
}

table ms_meta {
  id int [pk]
}

group series {
  ms_revenue
  ms_meta
}

select rows for series where org = :org
