// spec: §indexes, a plain single-column index, one column without where:, is the column setting unique or index and not an entry of an Indexes block, in a table or a partial
// want: is a plain single-column index: write the column setting unique on "email"
TablePartial stamps {
  created_at time.Time
  Indexes {
    created_at
  }
}

Table users {
  id    int    [pk]
  email string
  name  string
  ~stamps

  Indexes {
    email [unique]
    (name) [name: "idx_users_by_name"]
  }
}
