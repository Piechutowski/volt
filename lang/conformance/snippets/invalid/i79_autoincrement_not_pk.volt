// spec: §columns, autoincrement applies only to the rowid alias, the
// single-column int primary key; SQLite refuses it anywhere else
// want: only the rowid alias, the single-column int primary key, takes AUTOINCREMENT
table users {
  id int [pk]
  counter int [autoincrement]
}
