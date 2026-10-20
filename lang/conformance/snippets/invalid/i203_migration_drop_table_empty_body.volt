// spec: §migrations, the body of a drop table declares at least one column
// want: drop table "audit": the body must declare at least one column
package db

Table users {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
  drop table audit {
  }
}
