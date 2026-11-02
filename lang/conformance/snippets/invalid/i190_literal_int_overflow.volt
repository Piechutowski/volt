// spec: §predicates, a numeric literal must be representable in the column's storage type: an integer literal fits int64
// want: does not fit int
package db

table page_views {
  id int [pk]
  hits int
}

select big for page_views where hits > 9223372036854775808
