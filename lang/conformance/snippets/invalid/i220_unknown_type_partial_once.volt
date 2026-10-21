// spec: §columns, §tablepartial (a partial's unknown type is reported once, however many tables inject it)
// want: unknown type "int64"
TablePartial stamped {
  version int64 [not null]
}

Table authors {
  id int [pk, increment]
  ~stamped
}

Table posts {
  id int [pk, increment]
  ~stamped
}
