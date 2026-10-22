// spec: §tablepartial, an index is not overridden: one a partial injects may not repeat the key of an index of the table or of another injection, the same partial injected twice included
// want: which has an index on that key already
TablePartial emails {
  email string
  name  string
  Indexes {
    (email, name) [unique]
  }
}
TablePartial stamps {
  created_at time.Time
  updated_at time.Time
  Indexes {
    (created_at, updated_at)
  }
}
TablePartial stamps_again {
  Indexes {
    (created_at, updated_at)
  }
}
Table users {
  id int [pk]
  ~emails
  ~stamps
  ~stamps_again
  Indexes {
    (email, name)
  }
}
Table posts {
  id int [pk]
  ~stamps
  ~stamps
}
