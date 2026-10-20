// spec: §migrations, a migration name is four or more digits, two underscores and a label
// want: is not four or more digits, two underscores and a label
package db

Table users {
  id int [pk]
}

Migration 001__init {
  create table users {
    id int [pk]
  }
}
