// spec: §groups — '-' is not a group operator; set difference is spelled '\'
// want: set difference is spelled '\'
package db

Table ms_revenue {
  id int [pk]
}

Table ms_usage {
  id int [pk]
}

Group series {
  ms_revenue
  ms_usage
}

Group narrow = series - ms_usage
