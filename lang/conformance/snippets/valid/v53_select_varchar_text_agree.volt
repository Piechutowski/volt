// spec: §V11.4 — agreement is judged on the Go type: varchar and text agree
package db

Table ms_revenue {
  id integer [pk]
  org varchar(64) [not null]
}

Table ms_usage {
  id integer [pk]
  org text [not null]
}

Group series {
  ms_revenue
  ms_usage
}

Select by_org for series where org = :org
