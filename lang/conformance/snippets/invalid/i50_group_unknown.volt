// spec: §groups, a group term must name a table or a group
// want: no such table or group
package db

Table ms_revenue {
  id int [pk]
}

Group series {
  ms_revenue
  ms_revenu
}
