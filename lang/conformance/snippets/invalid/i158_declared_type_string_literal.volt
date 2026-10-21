// spec: §predicates: a literal against a declared type has the class of the storage type: a string against int storage is an error
// want: cannot be compared with the string
package db

Type Money int

Table orders {
  id    int   [pk, increment]
  price Money [not null]
}

Select odd for orders where price = 'x'
