// spec: §program-structure — representative complete schema
Project blog {
  database_type: 'PostgreSQL'
  Note: 'Blogging platform'
}

Enum post_status {
  draft
  published [note: 'visible to readers']
}

TablePartial timestamps {
  created_at time.Time [default: `now()`, not null]
  updated_at time.Time
}

Table users as U [headercolor: #175EE8] {
  id int [pk, increment]
  username string [not null, unique]
  ~timestamps
  Note: 'Registered users'

  indexes {
    username [unique, name: 'idx_username']
  }
}

Table posts {
  id int [pk]
  title string [not null]
  status post_status [default: post_status.draft]
  author_id int [not null, ref: > U.id]
  ~timestamps

  validators {
    TitleShape(title)
  }

  records (id, title, status, author_id) {
    1, 'Hello', post_status.published, 1
  }
}

TableGroup content [color: #cccccc] {
  posts
}

Note todo [color: #F4D03F] { 'add comments table' }

DiagramView main {
  Tables { * }
}
