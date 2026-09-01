// spec: §V11.7 — no duplicates in the projection list
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
}

Select summary (site, site) for page_views
