// spec: §program-structure — representative complete schema
Project blog [note: 'Blogging platform'] {
  database_type: 'PostgreSQL'
}

Enum post_status {
  draft
  published [note: 'visible to readers']
}

TablePartial timestamps {
  created_at time.Time [default: `now()`]
  updated_at time.Time
}

Table users [note: 'Registered users'] {
  id int [pk, increment]
  username string [unique]
  ~timestamps

  indexes {
    username [unique, name: 'idx_username']
  }
}

Table posts {
  id int [pk]
  title string
  status post_status [default: post_status.draft]
  author_id int [ref: users.id]
  ~timestamps

  validators {
    TitleShape(title)
  }

  records (id, title, status, author_id, updated_at) {
    1, 'Hello', post_status.published, 1, '2024-01-15 10:30:00'
  }
}

Group content {
  posts
}
