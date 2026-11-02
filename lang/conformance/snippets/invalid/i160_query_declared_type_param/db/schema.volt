package db

type money.Money int from "example.com/app/money"

table products {
  id    int         [pk, increment]
  price money.Money
}

select costing for products where price = :price
