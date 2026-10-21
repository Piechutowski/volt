// spec: §migrations, a create table body declares at least one column
// want: must declare at least one column
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
  }
}
