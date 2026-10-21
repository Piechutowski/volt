// spec: §migrations, type: is not a migration index setting
// want: type: names an index method
package db

Table users {
  id   int    [pk]
  name string [null]

  indexes {
    name [type: hash]
  }
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]
  }
  create index users name [type: hash]
}
