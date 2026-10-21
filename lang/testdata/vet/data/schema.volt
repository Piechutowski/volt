package data

Table "users" {
  "id" int [pk]
  email string [unique]
  foo_bar int
  total int
  status post_status
  new int
  indexes {
    id [unique]
    email [unique]
    (email, foo_bar)
    (email, foo_bar)
  }
}

Project blog [note: 'the blog schema'] {
  database_type: 'SQLite'
}

Enum post_status {
  draft
  published
}
Enum orphan_status { a }
Enum core_kind { widget }

TablePartial stamps {
  id int
  created_at time.Time
}
TablePartial audit_a {
  updated_at time.Time
}
TablePartial audit_b {
  updated_at time.Time
}
TablePartial orphan_partial { x int }
TablePartial owned {
  owner_id string [ref: users.id]
}

Table posts {
  ~stamps
  id string
  title string
  ~owned
  ~audit_a
  ~audit_b
  draft int [null]
  key int [pk]
}

Table core_things {
  id int [pk]
  k core_kind
}

Table naked {
  id int
  indexes {
  }
}
