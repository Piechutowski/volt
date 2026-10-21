// spec: §predicates, a numeric literal must be representable in the column's storage type: a float literal is finite
// want: does not fit float64
package db

Table page_views {
  id int [pk]
  score float64 [not null]
}

Select hot for page_views where score > 1e400
