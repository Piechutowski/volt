// spec: §groups, a group term must name a table or a group
// want: no such table or group
package db

table ms_revenue {
  id int [pk]
}

group series {
  ms_revenue
  ms_revenu
}
