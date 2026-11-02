// spec: §groups — block and algebra group forms, overlap allowed
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

table ks_costs {
  id int [pk]
  org string
  year int
}

group series {
  ms_revenue
  ms_usage
}

group wide = series + ks_costs
group narrow = wide \ ms_usage
