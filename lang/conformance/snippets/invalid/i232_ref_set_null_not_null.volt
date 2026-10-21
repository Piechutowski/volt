// spec: §relationships-ref, set null needs every foreign-key column to carry null, set default needs null or a default:
// want: set null on foreign-key column
Table users {
  id int [pk]
}

Table posts {
  id          int [pk]
  author_id   int
  reviewer_id int
  editor_id   int [default: 1]
}

Ref: posts.author_id > users.id [delete: set null]
Ref: posts.reviewer_id > users.id [update: set default]
Ref: posts.editor_id > users.id [delete: set default, update: set null]
