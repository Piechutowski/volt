// spec: §program-structure — representative complete schema
project blog [note: "Blogging platform"] {
  database_type: "SQLite"
}

type PostState string

table_partial timestamps {
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  updated_at time.Time
}

table users [note: "Registered users"] {
  id int [pk, increment]
  username string [unique]
  ~timestamps
}

table posts {
  id int [pk]
  title string
  status PostState [default: "draft"]
  author_id int [ref: users.id]
  ~timestamps

  validators {
    TitleShape(title)
  }

  records (id, title, status, author_id, updated_at) {
    1, "Hello", "published", 1, "2024-01-15 10:30:00"
  }
}

group content {
  posts
}
