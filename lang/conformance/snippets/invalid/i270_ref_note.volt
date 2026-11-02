// spec: §notes, §relationships-ref, a relationship carries no note: delete and update are its only settings
// want: unknown setting "note"
table users {
  id int [pk]
}
table posts {
  id      int [pk]
  user_id int
}
ref: posts.user_id > users.id [note: "the author"]
