// spec: §V9.3 — '-' is not a group operator; set difference is spelled '\'
// want: set difference is spelled '\'
package db

Table ms_revenue {
  id integer [pk]
}

Table ms_usage {
  id integer [pk]
}

Group series {
  ms_revenue
  ms_usage
}

Group narrow = series - ms_usage
