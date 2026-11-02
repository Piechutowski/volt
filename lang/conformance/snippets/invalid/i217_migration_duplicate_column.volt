// spec: §migrations, column names are unique within a shape body
// want: duplicate column name "name"
package db

table users {
  id   int    [pk]
  name string [null]
}

migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]
    name string [null]
  }
}
