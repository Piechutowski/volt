package data

Table "users" {
  "id" int [pk]
  email string [unique]
  foo_bar int
  total int
  status post_status [not null]
  new int
  indexes {
    id [unique]
    email [unique]
    (email, foo_bar)
    (email, foo_bar)
  }
}

Project blog {
  database_type: 'SQLite'
  Note: 'first'
  Note: 'second'
}

Enum post_status {
  draft
  published
}
Enum orphan_status { a }
Enum core.kind { widget }

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
  owner_id string [ref: > users.id]
}

Table posts as P {
  ~stamps
  id string
  title string
  ~owned
  ~audit_a
  ~audit_b
  draft int [null]
  key int pk
}

Table core.things {
  id int [pk]
  k core.kind
}

Table naked {
  id int
  indexes {
  }
}

TableGroup g {
}
