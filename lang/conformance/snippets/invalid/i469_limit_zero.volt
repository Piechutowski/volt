// spec: §selects-over-groups, a limit literal is at least 1
// want: keeps at least 1 row, not 0
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: 0]
