// spec: §columns, a declared type stored as int is the rowid alias but an ordinary key, never the database-assigned key that alone takes autoincrement
// want: a declared type stored as int is the rowid alias but an ordinary key
type ID int
table t {
  id ID [pk, autoincrement]
}
