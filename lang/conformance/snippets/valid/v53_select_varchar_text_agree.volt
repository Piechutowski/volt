// spec: §selects-over-groups — agreement is judged on the Go type: varchar and text agree
package db

Table ms_revenue {
  id int [pk]
  org string [not null]
}

Table ms_usage {
  id int [pk]
  org string [not null]
}

Group series {
  ms_revenue
  ms_usage
}

Select by_org for series where org = :org
