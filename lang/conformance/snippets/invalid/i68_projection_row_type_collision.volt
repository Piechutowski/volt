// spec: §selects-over-groups — a minted row type must not collide with a model
package db

Table page_views {
  id int [pk]
  site string
}

Table summaries {
  id int [pk]
  site string
}

Select summary (site) for page_views
