// spec: §columns, §default-values, §doc-comments, pk, autoincrement, unique, null, a doc comment and typed defaults
table t {
  id int [pk, autoincrement]
  // source
  a string [unique, default: "direct"]
  b int [null, default: 123]
  c float64 [default: 123.456]
  d bool [default: false]
  e string [null]
  f time.Time [default: `datetime('now', '-5 days')`]
  g int
}
