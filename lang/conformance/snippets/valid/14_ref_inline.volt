// spec: §relationships-ref, the inline form on the foreign-key column; a one-to-one is unique plus a ref
Table users {
  id int
}
Table posts { user_id int [ref: users.id] }
Table reviews { user_id int [null, ref: users.id] }
Table infos {
  user_id int [unique, ref: users.id]
}
