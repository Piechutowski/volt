// spec: §selects-over-groups, a select that aggregates without group: yields one row, which limit: cannot cap
// want: which limit: has nothing to cap
package db

table page_views {
  id   int [pk]
  hits int
}

select totals (count() as n, sum(hits) as total) for page_views [limit: 1]
