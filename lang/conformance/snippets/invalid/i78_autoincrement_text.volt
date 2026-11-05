// spec: §columns, autoincrement is valid on the database-assigned key alone, and a column that is not int is never the rowid alias
// want: a column that is not int is never the rowid alias
table users {
  code string [pk, autoincrement]
}
