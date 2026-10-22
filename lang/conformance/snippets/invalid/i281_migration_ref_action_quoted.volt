// spec: §migrations, a migration column's referential action is a word: a quoted action and a missing one are reported once each
// want: "delete" must be a referential action: cascade, restrict, set_null or set_default
package db

Table users {
  id int [pk]
}

Table posts {
  id      int [pk]
  user_id int [null]
}

Ref: posts.user_id > users.id

Migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id      int [pk]
    user_id int [null, ref: users.id, delete: 'set null', update]
  }
}
