// spec: §V11.4 — every member must have every referenced column
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
}

Table ms_meta {
  id integer [pk]
}

Group series {
  ms_revenue
  ms_meta
}

Select rows for series where org = :org
