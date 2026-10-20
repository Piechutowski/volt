// The fixture's migrations: they fold to schema.volt (spec §the-fold),
// which 'volt gen' verifies before it writes, and volt_migrations.go
// carries them for the tests that build a database through Migrate.
package itest

Migration 0001__users_and_orders {
  create table users {
    id         int       [pk, increment]
    email      string    [not null, unique]
    name       string    [not null]
    bio        string
    created_at time.Time [not null, default: `CURRENT_TIMESTAMP`]
  }
  create table orders {
    id        int       [pk, increment]
    user_id   int       [not null, ref: > users.id]
    status    string    [not null, default: 'pending']
    total     float64   [not null]
    tip       int       [not null, default: 0]
    fee       int       [not null]
    placed_at time.Time
  }
  create table user_tags {
    user_id int    [not null, ref: > users.id]
    tag     string [not null]

    indexes {
      (user_id, tag) [pk]
    }
  }
}

Migration 0002__metrics {
  create table page_views {
    id   int    [pk, increment]
    site string [not null]
    day  int    [not null]
    hits int    [not null, default: 0]
  }
  create table link_clicks {
    id     int    [pk, increment]
    site   string [not null]
    day    int    [not null]
    target string [not null, default: '']
  }
}
