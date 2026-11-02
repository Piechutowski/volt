// spec: §tablepartial, an index is not overridden: one a partial injects may not repeat the key of an index of the table or of another injection, the same partial injected twice included
// want: which has an index on that key already
table_partial emails {
  email string
  name  string
  indexes {
    (email, name) [unique]
  }
}
table_partial stamps {
  created_at time.Time
  updated_at time.Time
  indexes {
    (created_at, updated_at)
  }
}
table_partial stamps_again {
  indexes {
    (created_at, updated_at)
  }
}
table users {
  id int [pk]
  ~emails
  ~stamps
  ~stamps_again
  indexes {
    (email, name)
  }
}
table posts {
  id int [pk]
  ~stamps
  ~stamps
}
