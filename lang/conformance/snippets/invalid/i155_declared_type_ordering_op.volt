// spec: §predicates: a declared type is not orderable: < on it is an error
// want: compares with =, !=, in and is null only
package db

Type money.Money int from "example.com/app/money"

Table orders {
  id    int   [pk, increment]
  price money.Money
}

Select cheap for orders where price < :limit
