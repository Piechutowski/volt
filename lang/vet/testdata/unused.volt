// analyzers: unusedpartial
TablePartial used_partial { created_at time.Time }
TablePartial orphan_partial { x int } //WANT unusedpartial

Table users {
  id int
  ~used_partial
}
Table posts {
  id int
}
Ref: posts.id > users.id
