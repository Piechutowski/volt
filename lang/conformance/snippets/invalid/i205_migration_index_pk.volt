// spec/migrations, spec/indexes, pk is a column setting and no index setting, in an index_create as in a shape body, where it is the one error of its entry
// want: pk is a column setting, not an index setting
package db

table users {
  id int [pk]
}

migration 0001__init {
  table_create users {
    id   int [null]
    name string

    indexes {
      (id, name) [pk]
      name [pk]
    }
  }
  index_create users id [pk]
}
