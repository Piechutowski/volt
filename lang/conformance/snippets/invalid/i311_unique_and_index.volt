// spec: §columns, §indexes, unique and index are the two kinds of a column's index and not both: unique already indexes the column
// want: "unique" and "index" cannot both be set
Table users {
  id    int    [pk]
  email string [unique, index]
}
