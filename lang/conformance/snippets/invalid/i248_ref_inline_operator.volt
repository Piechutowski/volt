// spec: §relationships-ref, an inline ref names its endpoint with no operator: ref: users.id
// want: relationship endpoint
table users { id int [pk] }
table posts {
  user_id int [ref: > users.id]
}
