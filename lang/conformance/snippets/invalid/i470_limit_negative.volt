// spec: §selects-over-groups, a limit literal is at least 1; SQLite reads a negative limit as none
// want: keeps at least 1 row, not -1
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: -1]
