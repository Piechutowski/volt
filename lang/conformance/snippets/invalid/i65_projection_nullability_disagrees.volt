// spec: §selects-over-groups — agreement is on the generated field type, nullability
// included: text and text [not null] disagree
package db

Table page_views {
  id int [pk]
  site string [not null]
}

Table link_clicks {
  id int [pk]
  site string
}

Group metrics {
  page_views
  link_clicks
}

Select summary (site) for metrics
