// spec: §migrations, a flat body with a drop table without a body has no inverse
// want: drop table audit has no body
package db

table users {
  id int [pk]
}

migration 0001__init {
  create table users {
    id int [pk]
  }
  drop table audit
}
