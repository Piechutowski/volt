// spec: §selects-over-groups — no duplicates in the projection list
package db

table page_views {
  id int [pk]
  site string
}

select summary (site, site) for page_views
