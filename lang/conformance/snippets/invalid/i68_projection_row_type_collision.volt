// spec: §V11.7 — a minted row type must not collide with a model
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
}

Table summaries {
  id integer [pk]
  site varchar [not null]
}

Select summary (site) for page_views
