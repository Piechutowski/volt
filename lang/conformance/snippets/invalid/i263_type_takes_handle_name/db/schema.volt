// spec: §types, §projects-and-packages, a bare declared type is named apart from the names generated code declares in the package: OrderStatus is the column handle of orders.status
// want: takes the Go name of the column handle for orders.status
package db

Type OrderStatus string

Table orders {
  id     int         [pk, increment]
  status OrderStatus
}
