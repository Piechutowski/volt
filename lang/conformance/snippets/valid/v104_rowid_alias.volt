// spec: §columns, §records-sample-data, §mapping-to-sqlite-ddl, a single-column key of the built-in int that is no foreign key is the database-assigned key with or without autoincrement: records may omit it, while a declared type stored as int is the rowid alias too but an ordinary key, which records list
type Code int

table posts {
  id    int [pk]
  title string

  records (title) {
    "first"
  }
}

table events {
  id   int [pk, autoincrement]
  name string

  records (name) {
    "started"
  }
}

table tickets {
  code  Code [pk]
  label string

  records (code, label) {
    7, "seven"
  }
}
