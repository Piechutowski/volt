// spec: §selects-over-groups: a declared type cannot be ordered
// want: a declared type cannot be ordered
package db

type money.Money int from "example.com/app/money"

table orders {
  id    int   [pk, increment]
  price money.Money
}

select sorted for orders [order: (price asc)]
