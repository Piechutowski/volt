// spec: §selects-over-groups — a listed column must exist in every member
package db

table page_views {
  id int [pk]
  site string
  hits int
}

table link_clicks {
  id int [pk]
  site string
}

group metrics {
  page_views
  link_clicks
}

select summary (site, hits) for metrics
