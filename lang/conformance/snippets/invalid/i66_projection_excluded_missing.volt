// spec: §selects-over-groups — the algebra must say something true: an excluded
// column must exist in every member
package db

table page_views {
  id int [pk]
  site string
}

select public (* \ hits) for page_views
