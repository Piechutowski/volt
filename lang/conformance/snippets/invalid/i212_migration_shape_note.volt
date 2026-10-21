// spec: §migrations, a Note is not part of a shape body
// want: a migration's shape body holds columns and indexes only
package db

Table users {
  id int [pk]

  Note: 'people'
}

Migration 0001__init {
  create table users {
    id int [pk]

    Note: 'people'
  }
}
