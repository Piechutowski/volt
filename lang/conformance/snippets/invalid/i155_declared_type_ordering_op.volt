// spec: §predicates: a declared type is not orderable: < on it is an error
// want: compares with =, !=, in and is null only
package db

type money.Money int from "example.com/app/money"

table orders {
  id    int   [pk, increment]
  price money.Money
}

select cheap for orders where price < :limit
