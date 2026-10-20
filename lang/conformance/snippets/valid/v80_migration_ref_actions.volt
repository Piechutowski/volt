// spec: §migrations, a migration column's ref: takes > or - and may carry delete: and update: with the actions of §relationships-ref; an added ref: column defaults to null
package db

Table users {
  id int [pk, increment]
}

Table profiles {
  id      int [pk, increment]
  user_id int [not null, unique, ref: - users.id]
}

Table posts {
  id      int [pk, increment]
  user_id int [not null]
  editor  int
}

Ref: posts.user_id > users.id [delete: cascade, update: restrict]
Ref: posts.editor > users.id [delete: set null]

Migration 0001__init {
  create table users {
    id int [pk, increment]
  }
  create table profiles {
    id      int [pk, increment]
    user_id int [not null, unique, ref: - users.id]
  }
  create table posts {
    id      int [pk, increment]
    user_id int [not null, ref: > users.id, delete: cascade, update: restrict]
  }
}

Migration 0002__posts_editor {
  alter table posts {
    add editor int [default: null, ref: > users.id, delete: set null]
  }
}
