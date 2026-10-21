// spec: §columns, §selects-over-groups, §predicates (an unknown type silences only what depends on it)
// want: every column states its direction
package db

Table page_views {
  id   int    [pk, increment]
  hits int32
  body []byte
}

Table link_clicks {
  id   int    [pk, increment]
  body int64
}

Group pages {
  page_views
  link_clicks
}

Select undirected for page_views [order: (hits)]
Select sideways for page_views [order: (hits sideways)]
Select twice for page_views [order: (hits desc, hits asc)]
Select raw for pages where body = :b
Select raw_order for pages [order: (body desc)]
