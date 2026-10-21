// spec: §groups — a parenthesized set applies one name at a time, so re-adding through it is still an error
// want: already a member
package db

Table ms_revenue {
  id int [pk]
}

Table ms_usage {
  id int [pk]
}

Group Metrics {
  ms_revenue
  ms_usage
}

Group wide = Metrics + (ms_revenue)
