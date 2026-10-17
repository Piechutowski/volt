// spec: §selects-over-groups — no member may end up with zero columns
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
}

Select nothing (* \ (id, site)) for page_views
