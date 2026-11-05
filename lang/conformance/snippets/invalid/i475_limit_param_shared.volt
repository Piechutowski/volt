// spec: §selects-over-groups, the limit parameter names no other parameter of the select
// want: is the limit and a parameter of the select besides
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views where hits >= :n [order: (hits desc, id asc), limit: :n]
