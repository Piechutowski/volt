// The fixture's migrations: they fold to schema.volt (spec/the-fold),
// which 'volt gen' verifies before it writes, and volt_migrations.go
// carries them for the tests that build a database through Migrate.

package itest

migration 0001__users_and_orders {
  table_create users {
    id         int       [pk]
    email      string    [unique]
    name       string
    bio        string    [null]
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
  table_create orders {
    id        int       [pk]
    user_id   int       [ref: users.id]
    status    string    [default: "pending"]
    total     float64
    tip       int       [default: 0]
    fee       int
    placed_at time.Time [null]
  }
  table_create user_tags {
    user_id int    [pk, ref: users.id]
    tag     string [pk]
  }
}

migration 0002__metrics {
  table_create page_views {
    id   int    [pk]
    site string
    day  int
    hits int    [default: 0]
  }
  table_create link_clicks {
    id     int    [pk]
    site   string
    day    int
    target string [default: ""]
  }
}

migration 0003__site_flags {
  table_create site_flags {
    id       int     [pk]
    featured bool
    pinned   bool    [null]
    ratio    float64
    weight   float64 [null]
  }
}

migration 0004__attachments_and_blobs {
  table_create attachments {
    id     int    [pk]
    name   string [unique]
    body   []byte
    thumb  []byte [null]
    digest []byte
  }
  table_create blobs {
    hash []byte [pk]
    tag  []byte [unique]
    size int    [default: 0]
  }
}

migration 0005__daily_hits_and_invites {
  table_create daily_hits {
    day  int    [pk]
    hits int    [default: 0]
    site string [pk]
  }
  table_create invites {
    id         int    [pk]
    user_email string [ref: users.email]
    code       string [unique]
  }
}

// An index and a unique column added to a table that has rows, with no
// rebuild: each add is its ADD COLUMN and its CREATE INDEX
// (spec/migrations).

migration 0006__daily_hits_label_and_token {
  table_alter daily_hits {
    add label string [index, default: ""]
    add token string [null, unique]
  }
}

migration 0007__post_flags {
  table_create post_flags {
    id       int    [pk]
    reviewed string
    approved string [null]
  }
}

migration 0008__metric_events {
  table_create metric_events {
    id   int    [pk, autoincrement]
    name string
  }
}

migration 0009__batches {
  table_create batches {
    opened_at time.Time [pk]
    label     string    [default: ""]
  }
  table_create batch_items {
    id        int       [pk]
    opened_at time.Time [ref: batches.opened_at]
  }
}
