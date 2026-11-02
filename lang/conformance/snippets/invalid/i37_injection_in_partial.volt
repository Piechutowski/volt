// spec: §tablepartial — partial body has no nested injections
table_partial a { id int }
table_partial b {
  ~a
  name string
}
