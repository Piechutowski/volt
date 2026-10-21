// spec: §tablepartial, an index is not overridden: one a partial injects may not repeat the key of an index of the table or of another injection, the same partial injected twice included
// want: which has an index on that key already
TablePartial emails {
  email string
  indexes {
    email [unique]
  }
}
TablePartial stamps {
  created_at time.Time
  indexes {
    created_at
  }
}
TablePartial stamps_again {
  indexes {
    created_at
  }
}
Table users {
  id int [pk]
  ~emails
  ~stamps
  ~stamps_again
  indexes {
    email
  }
}
Table posts {
  id int [pk]
  ~stamps
  ~stamps
}
