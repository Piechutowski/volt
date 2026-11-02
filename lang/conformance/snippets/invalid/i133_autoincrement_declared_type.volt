// spec: §columns, only a key of the built-in int is the rowid alias, a declared type stored as int is not
// want: a declared type stored as int is not
type ID int
table t {
  id ID [pk, autoincrement]
}
