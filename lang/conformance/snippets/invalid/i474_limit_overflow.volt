// spec: §selects-over-groups, a limit literal fits int64
// want: does not fit int64
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc), limit: 9223372036854775808]
