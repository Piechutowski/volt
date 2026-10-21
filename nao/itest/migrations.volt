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
    status    string    [default: 'pending']
    total     float64
    tip       int       [default: 0]
    fee       int
    placed_at time.Time [null]
  }
  create table user_tags {
    user_id int    [ref: users.id]
    tag     string

    indexes {
      (user_id, tag) [pk]
    }
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
    target string [default: '']
  }
}
