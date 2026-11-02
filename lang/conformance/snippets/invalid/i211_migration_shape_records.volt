// spec: §migrations, a records block is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

table users {
  id   int    [pk]
  name string [null]
}

migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]

    records {
      1, "root"
    }
  }
}
