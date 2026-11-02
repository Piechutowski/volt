// spec: §migrations, a migration column's ref: takes > or - and may carry delete: and update: with the actions of §relationships-ref; an added ref: column defaults to null
package db

table users {
  id int [pk]
}

table profiles {
  id      int [pk]
  user_id int [unique, ref: users.id]
}

table posts {
  id      int [pk]
  user_id int
  editor  int [null]
}

ref: posts.user_id > users.id [delete: cascade, update: restrict]
ref: posts.editor > users.id [delete: set_null]

migration 0001__init {
  create table users {
    id int [pk]
  }
  create table profiles {
    id      int [pk]
    user_id int [unique, ref: users.id]
  }
  create table posts {
    id      int [pk]
    user_id int [ref: users.id, delete: cascade, update: restrict]
  }
}

migration 0002__posts_editor {
  alter table posts {
    add editor int [null, ref: users.id, delete: set_null]
  }
}
