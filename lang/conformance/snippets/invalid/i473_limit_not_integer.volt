// spec: §selects-over-groups, limit: takes a positive integer or a parameter, nothing else
// want: limit: takes a positive integer or a parameter
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: 2.5]
