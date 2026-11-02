// spec: §selects-over-groups — no member may end up with zero columns
package db

table page_views {
  id int [pk]
  site string
}

select nothing (* \ (id, site)) for page_views
