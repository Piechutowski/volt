// spec/migrations, a migration column's referential action is a word: a quoted action and a missing one are reported once each
// want: "delete" must be a referential action: cascade, restrict, null_set or default_set
package db

table users {
  id int [pk]
}

table posts {
  id      int [pk]
  user_id int [null]
}

ref: posts.user_id > users.id

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_create posts {
    id      int [pk]
    user_id int [null, ref: users.id, delete: "set null", update]
  }
}
