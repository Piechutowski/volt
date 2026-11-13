// spec: §projects-and-packages, a data package in lib__db declaring db
package db

table customers {
  id    int    [pk]
  email string [unique]
}
