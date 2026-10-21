// spec: §migrations, required describes Go and is not a migration setting
// want: required describes Go-side validation
package db

Table users {
  id    int    [pk]
  email string [not null, required]
}

Migration 0001__init {
  create table users {
    id    int    [pk]
    email string [not null, required]
  }
}
