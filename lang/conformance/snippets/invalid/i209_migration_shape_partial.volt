// spec: §migrations, a partial injection is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

table_partial timestamps {
  created time.Time
}

table users {
  id int [pk]
  ~timestamps
}

migration 0001__init {
  create table users {
    id int [pk]
    ~timestamps
  }
}
