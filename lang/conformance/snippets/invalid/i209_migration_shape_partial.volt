// spec: §migrations, a partial injection is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

TablePartial timestamps {
  created time.Time [not null]
}

Table users {
  id int [pk]
  ~timestamps
}

Migration 0001__init {
  create table users {
    id int [pk]
    ~timestamps
  }
}
