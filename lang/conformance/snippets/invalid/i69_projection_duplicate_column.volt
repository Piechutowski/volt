// spec: §selects-over-groups — no duplicates in the projection list
package db

Table page_views {
  id int [pk]
  site string
}

Select summary (site, site) for page_views
