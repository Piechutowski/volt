// spec: §notes, §relationships-ref, a relationship carries no note: delete and update are its only settings
// want: unknown setting "note"
Table users {
  id int [pk]
}
Table posts {
  id      int [pk]
  user_id int
}
Ref: posts.user_id > users.id [note: 'the author']
