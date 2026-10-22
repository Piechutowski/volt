// spec: §migrations, a records block is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

Table users {
  id   int    [pk]
  name string [null]
}

Migration 0001__init {
  create table users {
    id   int    [pk]
    name string [null]

    Records {
      1, "root"
    }
  }
}
