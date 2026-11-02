// spec: §migrations, a create table body declares at least one column
// want: must declare at least one column
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
  }
}
