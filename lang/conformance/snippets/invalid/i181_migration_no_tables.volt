// spec: §migrations, a package that declares no tables declares no migrations
// want: declares no tables
package db

migration 0001__init {
  create table users {
    id int [pk]
  }
}
