package data

Table "users" {
  "id" integer [pk]
  email varchar [unique]
  foo_bar int
  total int
  status post_status [not null]
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
  created_at timestamp
}
TablePartial audit_a {
  updated_at timestamp
}
TablePartial audit_b {
  updated_at "timestamp with time zone"
}
TablePartial orphan_partial { x int }
TablePartial owned {
  owner_id bigint [ref: > users.id]
}

Table posts as P {
  ~stamps
  id varchar
  title varchar
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
