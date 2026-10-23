// spec: §migrations, add takes an expression default, NOT NULL or null, without a rebuild
package db

Table posts {
  id         int       [pk, increment]
  title      string
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
}

Migration 0001__init {
  create table posts {
    id    int    [pk, increment]
    title string
  }
}

Migration 0002__stamps {
  alter table posts {
    add created_at time.Time [default: `CURRENT_TIMESTAMP`]
    add seen_at    time.Time [null, default: `CURRENT_TIMESTAMP`]
  }
}
