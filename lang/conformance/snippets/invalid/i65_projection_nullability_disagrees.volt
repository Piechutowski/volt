// spec: §V11.7 — agreement is on the generated field type, nullability
// included: text and text [not null] disagree
package db

Table page_views {
  id integer [pk]
  site text [not null]
}

Table link_clicks {
  id integer [pk]
  site text
}

Group metrics {
  page_views
  link_clicks
}

Select summary (site) for metrics
