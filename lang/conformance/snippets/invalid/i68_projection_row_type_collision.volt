// spec: §selects-over-groups — a minted row type must not collide with a model
package db

Table page_views {
  id int [pk]
  site string [not null]
}

Table summaries {
  id int [pk]
  site string [not null]
}

Select summary (site) for page_views
