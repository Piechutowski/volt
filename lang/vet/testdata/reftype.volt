// analyzers: reftype
Table users {
  id int [pk]
}
Table posts {
  user_id string
  ok_id int
}
Ref: posts.user_id > users.id //WANT reftype
Ref: posts.ok_id > users.id
