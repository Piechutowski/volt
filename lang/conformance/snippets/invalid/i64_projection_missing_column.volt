// spec: §V11.7 — a listed column must exist in every member
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
  hits integer [not null]
}

Table link_clicks {
  id integer [pk]
  site varchar [not null]
}

Group metrics {
  page_views
  link_clicks
}

Select summary (site, hits) for metrics
