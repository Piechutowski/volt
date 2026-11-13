// Package db is the shop's customers, in the directory lib__db.
package db

table customers {
  id    int    [pk]
  email string [unique]
  name  string [default: ""]
}
