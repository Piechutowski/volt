// spec: §relationships-ref, an inline ref names its endpoint with no operator: ref: users.id
// want: relationship endpoint
Table users { id int [pk] }
Table posts {
  user_id int [ref: > users.id]
}
