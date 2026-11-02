// spec: §selects-over-groups — agreement is judged on the Go type: varchar and text agree
package db

table ms_revenue {
  id int [pk]
  org string
}

table ms_usage {
  id int [pk]
  org string
}

group series {
  ms_revenue
  ms_usage
}

select by_org for series where org = :org
