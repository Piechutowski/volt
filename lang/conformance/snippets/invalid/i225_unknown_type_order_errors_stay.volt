// spec: §columns, §selects-over-groups, §predicates (an unknown type silences only what depends on it)
// want: every column states its direction
package db

table page_views {
  id   int    [pk, increment]
  hits int32
  body []byte
}

table link_clicks {
  id   int    [pk, increment]
  body int64
}

group pages {
  page_views
  link_clicks
}

select undirected for page_views [order: (hits)]
select sideways for page_views [order: (hits sideways)]
select twice for page_views [order: (hits desc, hits asc)]
select raw for pages where body = :b
select raw_order for pages [order: (body desc)]
