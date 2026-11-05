// spec: §columns, §default-values, default: is an error on the rowid alias, the database-assigned key with or without autoincrement, a foreign key and a declared type stored as int alike, since SQLite ignores a DEFAULT there
// want: SQLite ignores a DEFAULT on the rowid alias
type Code int

table users {
  id int [pk, default: 1]
}

table events {
  id   int    [pk, autoincrement, default: 1]
  name string
}

table user_profiles {
  user_id int    [pk, ref: users.id, default: 1]
  bio     string
}

table tickets {
  code  Code   [pk, default: 7]
  label string
}
