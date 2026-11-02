// The fixture's migrations: they fold to schema.volt (spec §the-fold),
// which 'volt gen' verifies before it writes, and volt_migrations.go
// carries them for the tests that build a database through Migrate.
package itest

Migration 0001__users_and_orders {
  create table users {
    id         int       [pk, increment]
    email      string    [unique]
    name       string
    bio        string    [null]
    created_at time.Time [default: `CURRENT_TIMESTAMP`]
  }
  create table orders {
    id        int       [pk, increment]
    user_id   int       [ref: users.id]
    status    string    [default: "pending"]
    total     float64
    tip       int       [default: 0]
    fee       int
    placed_at time.Time [null]
  }
  create table user_tags {
    user_id int    [pk, ref: users.id]
    tag     string [pk]
  }
}

Migration 0002__metrics {
  create table page_views {
    id   int    [pk, increment]
    site string
    day  int
    hits int    [default: 0]
  }
  create table link_clicks {
    id     int    [pk, increment]
    site   string
    day    int
    target string [default: ""]
  }
}

Migration 0003__site_flags {
  create table site_flags {
    id       int     [pk, increment]
    featured bool
    pinned   bool    [null]
    ratio    float64
    weight   float64 [null]
  }
}

Migration 0004__attachments_and_blobs {
  create table attachments {
    id     int    [pk, increment]
    name   string [unique]
    body   []byte
    thumb  []byte [null]
    digest []byte
  }
  create table blobs {
    hash []byte [pk]
    tag  []byte [unique]
    size int    [default: 0]
  }
}

Migration 0005__daily_hits_and_invites {
  create table daily_hits {
    day  int    [pk]
    hits int    [default: 0]
    site string [pk]
  }
  create table invites {
    id         int    [pk, increment]
    user_email string [ref: users.email]
    code       string [unique]
  }
}

// An index and a unique column added to a table that has rows, with no
// rebuild: each add is its ADD COLUMN and its CREATE INDEX (spec
// §migrations).
Migration 0006__daily_hits_label_and_token {
  alter table daily_hits {
    add label string [index, default: ""]
    add token string [null, unique]
  }
}

Migration 0007__post_flags {
  create table post_flags {
    id       int    [pk, increment]
    reviewed string
    approved string [null]
  }
}
