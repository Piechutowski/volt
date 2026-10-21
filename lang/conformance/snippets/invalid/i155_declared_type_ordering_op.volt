// spec: §predicates: a declared type is not orderable: < on it is an error
// want: compares with =, !=, in and is null only
package db

Type Money int

Table orders {
  id    int   [pk, increment]
  price Money
}

Select cheap for orders where price < :limit
