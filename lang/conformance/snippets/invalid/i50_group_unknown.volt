// spec: §V9.2 — a group term must name a table, group or TableGroup
// want: no such table, group or TableGroup
package db

Table ms_revenue {
  id integer [pk]
}

Group series {
  ms_revenue
  ms_revenu
}
