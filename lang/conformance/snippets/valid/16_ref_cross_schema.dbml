// spec: §relationships-ref — cross-schema endpoints
Table core.users { id int }
Table blogging.posts {
  user_id int [ref: > core.users.id]
}
Table blogging.comments { user_id int }
Ref: blogging.comments.user_id > core.users.id
