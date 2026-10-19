// spec: §selects-over-groups — every member must have every referenced column
package db

Table ms_revenue {
  id int [pk]
  org string [not null]
}

Table ms_meta {
  id int [pk]
}

Group series {
  ms_revenue
  ms_meta
}

Select rows for series where org = :org
