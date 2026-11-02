// spec: §migrations, a migration name is unique within the package
// want: duplicate migration name
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
}

migration 0001__init {
  create index users id
}
