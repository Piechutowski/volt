// spec: §migrations, a migration name is unique within the package
// want: duplicate migration name
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
}

Migration 0001__init {
  create index users id
}
