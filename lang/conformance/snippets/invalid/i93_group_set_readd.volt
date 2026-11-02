// spec: §groups — a parenthesized set applies one name at a time, so re-adding through it is still an error
// want: already a member
package db

table ms_revenue {
  id int [pk]
}

table ms_usage {
  id int [pk]
}

group Metrics {
  ms_revenue
  ms_usage
}

group wide = Metrics + (ms_revenue)
