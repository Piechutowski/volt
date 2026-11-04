// spec: §selects-over-groups, §predicates, the limit parameter is never the blank identifier
// want: parameter name "_" does not work as a Go parameter
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc, id asc), limit: :_]
