// spec: §selects-over-groups — the data package whose default CRUD the resources bind
package db

table users {
  id    int [pk, increment]
  email string    [unique]
}

table tags {
  id   int [pk, increment]
  name string
}
