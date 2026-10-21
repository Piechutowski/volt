// spec: §selects-over-groups — the data package whose default CRUD the resources bind
package db

Table users {
  id    int [pk, increment]
  email string    [unique]
}

Table tags {
  id   int [pk, increment]
  name string
}
