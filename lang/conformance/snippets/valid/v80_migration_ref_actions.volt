// spec/migrations, a migration column's ref: takes > or - and may carry delete: and update: with the actions of spec/relationships-ref; an added ref: column defaults to null
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
ref: posts.editor > users.id [delete: null_set]

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_create profiles {
    id      int [pk]
    user_id int [unique, ref: users.id]
  }
  table_create posts {
    id      int [pk]
    user_id int [ref: users.id, delete: cascade, update: restrict]
  }
}

migration 0002__posts_editor {
  table_alter posts {
    add editor int [null, ref: users.id, delete: null_set]
  }
}
