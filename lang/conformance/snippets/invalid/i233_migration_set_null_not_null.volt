// spec: §migrations, §relationships-ref, a migration column's set_null needs null, set_default needs null or a default:
// want: set_null on foreign-key column
package db

table users {
  id int [pk]
}

table posts {
  id          int [pk]
  author_id   int [null]
  reviewer_id int [null]
}

ref: posts.author_id > users.id [delete: set_null]
ref: posts.reviewer_id > users.id [update: set_default]

migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id          int [pk]
    author_id   int [ref: users.id, delete: set_null]
    reviewer_id int [ref: users.id, update: set_default]
  }
}
