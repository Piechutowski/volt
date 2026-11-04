// spec: §selects-over-groups, §predicates, the limit parameter obeys the parameter name rules
// want: does not work as a Go parameter
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: :ctx]
