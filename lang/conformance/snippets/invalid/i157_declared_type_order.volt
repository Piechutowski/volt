// spec: §selects-over-groups: a declared type cannot be ordered
// want: a declared type cannot be ordered
package db

Type Money int

Table orders {
  id    int   [pk, increment]
  price Money
}

Select sorted for orders [order: (price asc)]
