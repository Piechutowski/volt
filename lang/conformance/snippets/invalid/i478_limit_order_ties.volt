// spec: §selects-over-groups, limit: needs a total order; an order that leaves ties keeps arbitrary rows at the cut
// want: takes limit: over an order that leaves ties: order: names no key of page_views
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: 10]
