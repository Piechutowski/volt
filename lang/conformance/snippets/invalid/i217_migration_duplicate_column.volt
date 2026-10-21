// spec: §migrations, column names are unique within a shape body
// want: duplicate column name "name"
package db

Table users {
  id   int    [pk]
  name string [null]
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]
    name string [null]
  }
}
