// nullable vs required foreign keys and other nullable shapes
Table users {
  id int [pk]
}
Table posts {
  id int [pk]
  author_id int [not null, ref: > users.id]
  editor_id int [ref: > users.id, note: 'NULL until an editor is assigned']
  reviewed_at time.Time
  meta string [note: 'nil slice expresses NULL; no pointer needed']
  raw []byte
}
