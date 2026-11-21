// spec/migrations, spec/relationships-ref, a migration column's null_set needs null, default_set needs null or a default:
// want: null_set on foreign-key column
package db

table users {
  id int [pk]
}

table posts {
  id          int [pk]
  author_id   int [null]
  reviewer_id int [null]
}

ref: posts.author_id > users.id [delete: null_set]
ref: posts.reviewer_id > users.id [update: default_set]

migration 0001__init {
  table_create users {
    id int [pk]
  }
  table_create posts {
    id          int [pk]
    author_id   int [ref: users.id, delete: null_set]
    reviewer_id int [ref: users.id, update: default_set]
  }
}
