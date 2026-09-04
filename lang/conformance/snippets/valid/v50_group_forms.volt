// spec: §V9 — block and algebra group forms, overlap allowed
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

Table ks_costs {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Group series {
  ms_revenue
  ms_usage
}

Group wide = series + ks_costs
Group narrow = wide \ ms_usage
