// spec: §columns, §relationships-ref, an inline ref carries no settings: a relationship with delete: or update: is a Ref element
// want: unknown setting "delete"
table users {
  id int [pk]
}
table posts {
  id      int [pk]
  user_id int [ref: users.id, delete: cascade]
}
