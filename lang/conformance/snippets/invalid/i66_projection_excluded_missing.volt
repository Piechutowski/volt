// spec: §selects-over-groups — the algebra must say something true: an excluded
// column must exist in every member
package db

Table page_views {
  id int [pk]
  site string
}

Select public (* \ hits) for page_views
