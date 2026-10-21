// spec: §predicates, §types: a declared type compares with =, !=, in and is null; a literal has the class of the storage type
package db

Type Money int

Table orders {
  id    int   [pk, increment]
  price Money [not null]
  tip   Money
}

Select priced for orders where price = :price and tip != :tip
Select among  for orders where price in :prices and tip is null
Select cheap  for orders where price = 5 or price in (1, 2)
