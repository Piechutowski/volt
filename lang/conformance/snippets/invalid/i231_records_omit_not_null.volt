// spec: §records-sample-data, a records column list names every NOT NULL column without a default, injected ones included
// want: do not list column
TablePartial stamped {
  stamped_by string
}

Table posts {
  id       int [pk]
  title    string
  subtitle string [null]
  views    int [default: 0]
  ~stamped

  Records (id, subtitle) {
    1, 'first'
  }
}

Table authors {
  id   int [pk, increment]
  name string
  bio  string [null]
}

Records authors(bio) {
  'writes'
}
