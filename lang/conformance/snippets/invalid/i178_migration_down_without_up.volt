// spec: §migrations, a down block follows an up block
// want: a down block follows an up block
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  down {
    drop table users
  }
}
