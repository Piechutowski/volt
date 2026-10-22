package data

Table users {
  id int [pk, unique]
  email string [unique]
  foo_bar int
  total int
  status PostStatus
  new int
  Indexes {
    email [unique, name: "idx_users_email_live", where: `total > 0`]
    (email, foo_bar)
    (email, foo_bar) [name: "idx_users_email_foo_bar_again"]
  }

  Validators {
    PostStatusKnown(status)
  }
}

Project blog [note: "the blog schema"] {
  database_type: "SQLite"
}

Type PostStatus string
Type CoreKind string

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
  k CoreKind

  Validators {
    CoreKindKnown(k)
  }
}

Table naked {
  id int
  Indexes {
  }
}
