// spec: §selects-over-groups — a listed column must exist in every member
package db

Table page_views {
  id int [pk]
  site string
  hits int
}

Table link_clicks {
  id int [pk]
  site string
}

Group metrics {
  page_views
  link_clicks
}

Select summary (site, hits) for metrics
