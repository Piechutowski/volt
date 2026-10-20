// spec: §escape-sequences — escape sequences including \uHHHH and unknown escapes
Table t {
  a string [note: 'tab\there']
  b string [note: 'unicode A char']
  c string [note: 'unknown \q escape yields q']
  d string [default: 'it\'s escaped']
}
