// spec: §V9.3 — adding a member twice is an error, the algebra must say something true
package db

Table ms_revenue {
  id integer [pk]
}

Group series {
  ms_revenue
}

Group wide = series + ms_revenue
