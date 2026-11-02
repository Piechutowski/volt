// spec: §migrations, a down block follows an up block
// want: a down block follows an up block
package db

table users {
  id int [pk]
}

migration 0001__init {
  down {
    drop table users
  }
}
