// spec: §predicates: a literal against a declared type has the class of the storage type: a string against int storage is an error
// want: cannot be compared with the string
package db

type money.Money int from "example.com/app/money"

table orders {
  id    int   [pk, increment]
  price money.Money
}

select odd for orders where price = "x"
