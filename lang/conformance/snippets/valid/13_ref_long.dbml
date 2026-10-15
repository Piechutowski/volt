// spec: §relationships-ref — long form (named) contains exactly one relationship
Table users { id int }
Table posts { user_id int }
Ref user_fk {
  posts.user_id > users.id [delete: cascade]
}
