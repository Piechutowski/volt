// spec: §V11.5 — a column appears at most once in order:
// want: appears twice in order
package db

Table page_views {
  id integer [pk]
  day integer [not null]
}

Select rows for page_views [order: (day desc, day asc)]
