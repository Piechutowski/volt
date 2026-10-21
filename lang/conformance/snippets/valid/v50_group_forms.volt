// spec: §groups — block and algebra group forms, overlap allowed
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

Table ks_costs {
  id int [pk]
  org string
  year int
}

Group series {
  ms_revenue
  ms_usage
}

Group wide = series + ks_costs
Group narrow = wide \ ms_usage
