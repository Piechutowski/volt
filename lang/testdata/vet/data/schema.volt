package data

table users {
  id int [pk, unique]
  email string [unique]
  foo_bar int
  total int
  status PostStatus
  new int
  indexes {
    email [unique, name: "idx_users_email_live", where: `total > 0`]
    (email, foo_bar)
    (email, foo_bar) [name: "idx_users_email_foo_bar_again"]
  }

  validators {
    PostStatusKnown(status)
  }
}

type PostStatus string
type CoreKind string

table_partial stamps {
  id int
  created_at time.Time
}
table_partial audit_a {
  updated_at time.Time
}
table_partial audit_b {
  updated_at time.Time
}
table_partial orphan_partial { x int }
table_partial owned {
  owner_id string [ref: users.id]
}

table posts {
  ~stamps
  id string
  title string
  ~owned
  ~audit_a
  ~audit_b
  draft int [null]
  key int [pk]
}

table core_things {
  id int [pk]
  k CoreKind

  validators {
    CoreKindKnown(k)
  }
}

table naked {
  id int
  indexes {
  }
}
