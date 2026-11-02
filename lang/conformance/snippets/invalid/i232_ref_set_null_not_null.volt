// spec: §relationships-ref, set_null needs every foreign-key column to carry null, set_default needs null or a default:
// want: set_null on foreign-key column
table users {
  id int [pk]
}

table posts {
  id          int [pk]
  author_id   int
  reviewer_id int
  editor_id   int [default: 1]
}

ref: posts.author_id > users.id [delete: set_null]
ref: posts.reviewer_id > users.id [update: set_default]
ref: posts.editor_id > users.id [delete: set_default, update: set_null]
