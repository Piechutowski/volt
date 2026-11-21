// analyzers: reftype

table users {
  id int [pk]
}
table posts {
  user_id string
  ok_id int
}
ref: posts.user_id > users.id //WANT reftype
ref: posts.ok_id > users.id
