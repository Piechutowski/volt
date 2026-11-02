// spec: §selects-over-groups — a minted row type must not collide with a model
package db

table page_views {
  id int [pk]
  site string
}

table summaries {
  id int [pk]
  site string
}

select summary (site) for page_views
