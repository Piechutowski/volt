// spec/relationships-ref, null_set needs every foreign-key column to carry null, default_set needs null or a default:
// want: null_set on foreign-key column
table users {
  id int [pk]
}

table posts {
  id          int [pk]
  author_id   int
  reviewer_id int
  editor_id   int [default: 1]
}

ref: posts.author_id > users.id [delete: null_set]
ref: posts.reviewer_id > users.id [update: default_set]
ref: posts.editor_id > users.id [delete: default_set, update: null_set]
