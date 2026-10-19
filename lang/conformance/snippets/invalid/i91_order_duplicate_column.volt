// spec: §selects-over-groups — a column appears at most once in order:
// want: appears twice in order
package db

Table page_views {
  id int [pk]
  day int [not null]
}

Select rows for page_views [order: (day desc, day asc)]
