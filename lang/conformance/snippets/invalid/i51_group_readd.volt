// spec: §groups — adding a member twice is an error, the algebra must say something true
package db

table ms_revenue {
  id int [pk]
}

group series {
  ms_revenue
}

group wide = series + ms_revenue
