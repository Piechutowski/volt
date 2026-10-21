// spec: §columns, §default-values, pk, increment, unique, null, note: and typed defaults
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
