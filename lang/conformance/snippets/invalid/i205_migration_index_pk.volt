// spec: §migrations, pk is not a setting of a standalone create index or drop index, since a primary key is a table constraint
// want: pk is not a setting of a standalone create index or drop index
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [null]
  }
  create index users id [pk]
}
