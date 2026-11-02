// spec: §selects-over-groups — agreement is on the generated field type, nullability
// included: string and string [null] disagree
package db

table page_views {
  id int [pk]
  site string
}

table link_clicks {
  id int [pk]
  site string [null]
}

group metrics {
  page_views
  link_clicks
}

select summary (site) for metrics
