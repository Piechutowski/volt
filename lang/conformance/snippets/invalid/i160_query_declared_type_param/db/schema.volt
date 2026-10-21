package db

Type money.Money int from 'example.com/app/money'

Table products {
  id    int         [pk, increment]
  price money.Money [not null]
}

Select costing for products where price = :price
