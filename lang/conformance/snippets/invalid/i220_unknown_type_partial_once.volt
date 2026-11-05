// spec: §columns, §tablepartial (a partial's unknown type is reported once, however many tables inject it)
// want: unknown type "int64"
table_partial stamped {
  version int64
}

table authors {
  id int [pk]
  ~stamped
}

table posts {
  id int [pk]
  ~stamped
}
