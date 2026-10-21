// spec: §migrations, column names are unique within a shape body
// want: duplicate column name "name"
package db

Table users {
  id   int    [pk]
  name string
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string
    name string
  }
}
