package db

type money.Money int from "example.com/app/money"

table products {
  id    int         [pk, increment]
  price money.Money
}

select priced for products where price in :prices
