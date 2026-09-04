// spec: §V9.3 — a parenthesized set applies one name at a time, so re-adding through it is still an error
// want: already a member
package db

Table ms_revenue {
  id integer [pk]
}

Table ms_usage {
  id integer [pk]
}

TableGroup Metrics {
  ms_revenue
  ms_usage
}

Group wide = Metrics + (ms_revenue)
