// spec: §migrations, a flat body with a drop table without a body has no inverse
// want: drop table audit has no body
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
  drop table audit
}
