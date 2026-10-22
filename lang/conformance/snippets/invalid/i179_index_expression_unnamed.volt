// spec: §indexes, an index with an expression in its key carries name:
// want: must carry name:
Table users {
  id    int [pk]
  email string

  Indexes {
    (`lower(email)`) [unique]
  }
}
