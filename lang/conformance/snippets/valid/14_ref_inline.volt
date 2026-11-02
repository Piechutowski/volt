// spec: §relationships-ref, the inline form on the foreign-key column; a one-to-one is unique plus a ref
table users {
  id int
}
table posts { user_id int [ref: users.id] }
table reviews { user_id int [null, ref: users.id] }
table infos {
  user_id int [unique, ref: users.id]
}
