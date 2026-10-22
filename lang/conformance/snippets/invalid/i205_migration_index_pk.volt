// spec: §migrations, §indexes, pk is a column setting and no index setting, in a create index as in a shape body, where it is the one error of its entry
// want: pk is a column setting, not an index setting
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id   int [null]
    name string

    Indexes {
      (id, name) [pk]
      name [pk]
    }
  }
  create index users id [pk]
}
