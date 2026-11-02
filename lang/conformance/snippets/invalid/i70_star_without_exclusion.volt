// spec: §selects-over-groups — a star projection needs at least one exclusion; drop
// the parens to select every column
package db

table page_views {
  id int [pk]
  site string
}

select all (*) for page_views
