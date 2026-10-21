// spec: §columns, §default-values — every column setting; repeatable check
Table t {
  id int [pk, increment]
  a string [unique, default: 'direct', note: 'source']
  b int [null, default: 123]
  c float64 [default: 123.456]
  d bool [default: false]
  e string [null]
  f time.Time [default: `now() - interval '5 days'`]
  g int
}
