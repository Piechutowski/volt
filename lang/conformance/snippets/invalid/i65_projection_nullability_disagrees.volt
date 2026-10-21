// spec: §selects-over-groups — agreement is on the generated field type, nullability
// included: string and string [null] disagree
package db

Table page_views {
  id int [pk]
  site string
}

Table link_clicks {
  id int [pk]
  site string [null]
}

Group metrics {
  page_views
  link_clicks
}

Select summary (site) for metrics
