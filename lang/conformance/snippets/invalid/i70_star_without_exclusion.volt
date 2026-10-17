// spec: §selects-over-groups — a star projection needs at least one exclusion; drop
// the parens to select every column
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
}

Select all (*) for page_views
