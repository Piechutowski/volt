// spec: §columns, autoincrement is SQLite's keyword of the rowid alias, which only an int key is
// want: only an int column can be the rowid alias
table users {
  code string [pk, autoincrement]
}
