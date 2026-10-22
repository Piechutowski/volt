// spec: §indexes, the key of a [pk] index is columns only
// want: a primary key is columns only
Table posts {
  slug    string
  version int

  Indexes {
    (`lower(slug)`, version) [pk]
  }
}
