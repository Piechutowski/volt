// nullable vs NOT NULL foreign keys and other nullable shapes
Table users {
  id int [pk]
}
Table posts {
  id int [pk]
  author_id int [ref: > users.id]
  editor_id int [null, ref: > users.id, note: 'NULL until an editor is assigned']
  reviewed_at time.Time [null]
  meta string [null, note: 'rt.Null[string] expresses NULL; no pointer needed']
  raw []byte [null]
}
