// spec: §migrations, delete: and update: on a migration column take the referential actions of §relationships-ref
// want: invalid referential action "explode" for 'delete'
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [null, ref: > users.id]
}

Migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id      int [pk]
    user_id int [null, ref: > users.id, delete: explode]
  }
}
