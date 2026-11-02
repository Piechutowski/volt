// spec: §groups — '-' is not a group operator; set difference is spelled '\'
// want: set difference is spelled '\'
package db

table ms_revenue {
  id int [pk]
}

table ms_usage {
  id int [pk]
}

group series {
  ms_revenue
  ms_usage
}

group narrow = series - ms_usage
