// spec: §program-structure — representative complete schema
Project blog [note: "Blogging platform"] {
  database_type: "SQLite"
}

Type PostState string

TablePartial timestamps {
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  updated_at time.Time
}

Table users [note: "Registered users"] {
  id int [pk, increment]
  username string [unique]
  ~timestamps
}

Table posts {
  id int [pk]
  title string
  status PostState [default: "draft"]
  author_id int [ref: users.id]
  ~timestamps

  Validators {
    TitleShape(title)
  }

  Records (id, title, status, author_id, updated_at) {
    1, "Hello", "published", 1, "2024-01-15 10:30:00"
  }
}

Group content {
  posts
}
