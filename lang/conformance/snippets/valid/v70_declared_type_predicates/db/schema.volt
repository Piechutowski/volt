// spec: §predicates, §types: a declared type compares with =, !=, in and is null; a literal has the class of the storage type
package db

type Money int

table orders {
  id    int   [pk]
  price Money
  tip   Money [null]
}

select priced for orders where price = :price and tip != :tip
select among  for orders where price in :prices and tip is null
select cheap  for orders where price = 5 or price in (1, 2)
