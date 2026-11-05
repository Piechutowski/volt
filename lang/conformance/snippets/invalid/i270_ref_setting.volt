// spec: §relationships-ref, a relationship takes delete:, update:, as: and reverse: only
// want: unknown setting "name"
table users {
  id int [pk]
}
table posts {
  id      int [pk]
  user_id int
}
ref: posts.user_id > users.id [name: "the author"]
