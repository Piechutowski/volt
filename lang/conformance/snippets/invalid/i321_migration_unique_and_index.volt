// spec: §migrations, §columns, unique and index are not both set on a migration column, in a create table body as in a schema table
// want: "unique" and "index" cannot both be set
package db

Table users {
  id    int    [pk]
  email string [unique]
}

Migration 0001__init {
  create table users {
    id    int    [pk]
    email string [unique, index]
  }
}
