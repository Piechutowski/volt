// spec: §indexes, a plain single-column index, one column without where:, is the column setting unique or index and not an entry of an Indexes block, in a table or a partial
// want: is a plain single-column index: write the column setting unique on "email"
table_partial stamps {
  created_at time.Time
  indexes {
    created_at
  }
}

table users {
  id    int    [pk]
  email string
  name  string
  ~stamps

  indexes {
    email [unique]
    (name) [name: "idx_users_by_name"]
  }
}
