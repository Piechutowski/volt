// spec: §migrations, ref: on a migration column takes > or -, since the declaring column holds the key
// want: ref: takes > or - in a migration
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [ref: > users.id]
}

Migration 0001__init {
  create table users {
    id int [pk, ref: < posts.user_id]
  }
  create table posts {
    id      int [pk]
    user_id int
  }
}
