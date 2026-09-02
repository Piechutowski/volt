// spec: §V11.7 — the algebra must say something true: an excluded
// column must exist in every member
package db

Table page_views {
  id integer [pk]
  site varchar [not null]
}

Select public (* - hits) for page_views
