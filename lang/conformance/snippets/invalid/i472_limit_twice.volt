// spec: §selects-over-groups, a select takes one limit:
// want: takes limit: twice
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc, id asc), limit: 10, limit: :limit]
