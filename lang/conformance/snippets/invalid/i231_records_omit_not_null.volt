// spec: §records-sample-data, a records column list names every NOT NULL column without a default, injected ones included
// want: do not list column
table_partial stamped {
  stamped_by string
}

table posts {
  id       int [pk]
  title    string
  subtitle string [null]
  views    int [default: 0]
  ~stamped

  records (id, subtitle) {
    1, "first"
  }
}

table authors {
  id   int [pk]
  name string
  bio  string [null]
}

records authors(bio) {
  "writes"
}
