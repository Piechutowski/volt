// analyzers: unusedpartial
table_partial used_partial { created_at time.Time }
table_partial orphan_partial { x int } //WANT unusedpartial

table users {
  id int
  ~used_partial
}
table posts {
  id int
}
ref: posts.id > users.id
