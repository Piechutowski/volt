// nullable vs NOT NULL foreign keys and other nullable shapes

table users {
  id int [pk]
}
table posts {
  id int [pk]
  author_id int [ref: users.id]
  // NULL until an editor is assigned
  editor_id int [null, ref: users.id]
  reviewed_at time.Time [null]
  // rt.Null[string] expresses NULL; no pointer needed
  meta string [null]
  raw []byte [null]
}
