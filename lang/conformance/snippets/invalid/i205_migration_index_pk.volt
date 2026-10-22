// spec: §migrations, §indexes, pk is a column setting and no index setting, in a create index as in a shape body
// want: pk is a column setting, not an index setting
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
