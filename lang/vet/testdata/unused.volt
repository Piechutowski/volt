// analyzers: unusedenum,unusedpartial
Enum used_status { a }
Enum orphan_status { a } //WANT unusedenum
Enum kind { widget }

TablePartial used_partial { created_at time.Time }
TablePartial orphan_partial { x int } //WANT unusedpartial

Table users {
  id int
  s used_status
  ~used_partial
}
Table posts {
  id int
}
Table things {
  k kind
}
Ref: posts.id > users.id
