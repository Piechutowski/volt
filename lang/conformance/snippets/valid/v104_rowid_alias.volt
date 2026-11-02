// spec: §columns, §records-sample-data, §mapping-to-sqlite-ddl, a single-column int key is the rowid alias with or without autoincrement: records may omit it, and a declared type stored as int is an ordinary key
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
