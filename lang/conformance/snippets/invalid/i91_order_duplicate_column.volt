// spec: §selects-over-groups — a column appears at most once in order:
// want: appears twice in order
package db

table page_views {
  id int [pk]
  day int
}

select rows for page_views [order: (day desc, day asc)]
