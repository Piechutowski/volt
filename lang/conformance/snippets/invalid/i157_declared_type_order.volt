// spec: §selects-over-groups: a declared type cannot be ordered
// want: a declared type cannot be ordered
package db

Type money.Money int from "example.com/app/money"

Table orders {
  id    int   [pk, increment]
  price money.Money
}

Select sorted for orders [order: (price asc)]
