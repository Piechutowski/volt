// spec: §columns — only an int column auto-increments, a declared type stored as int does not
// want: only an int column can auto-increment
type ID int
table t {
  id ID [pk, increment]
}
